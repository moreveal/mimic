use crate::ffi::{Handle, call};

/// Bounded readback: NUL-terminated property names in, u32 LE byte-length
/// followed by UTF-8 per property out. Buffer exhaustion is not owner failure.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn mimic_blitz_style_batch(
    handle: *mut Handle,
    id: u64,
    names: *const u8,
    names_len: usize,
    output: *mut u8,
    capacity: usize,
    written: *mut usize,
) -> i32 {
    if written.is_null()
        || (capacity != 0 && output.is_null())
        || (names_len != 0 && names.is_null())
        || names_len > 1024 * 1024
    {
        return -1;
    }
    let mut insufficient = false;
    let status = call(handle, |owner| {
        let bytes = if names_len == 0 {
            &[][..]
        } else {
            unsafe { std::slice::from_raw_parts(names, names_len) }
        };
        let text = std::str::from_utf8(bytes).map_err(|_| ())?;
        if !text.is_empty() && !text.ends_with('\0') {
            return Err(());
        }
        let properties: Vec<&str> = text.split_terminator('\0').collect();
        if properties.len() > 4096 || properties.iter().any(|name| name.is_empty()) {
            return Err(());
        }
        let values = owner.style_batch(id, &properties).map_err(|_| ())?;
        if values.len() != properties.len() {
            return Err(());
        }
        let mut size = 0usize;
        for value in &values {
            size = size.checked_add(4 + value.len()).ok_or(())?;
            if size > 64 * 1024 * 1024 {
                return Err(());
            }
        }
        unsafe {
            *written = size;
        }
        if size > capacity {
            insufficient = true;
            return Ok(());
        }
        let mut offset = 0;
        for value in values {
            unsafe {
                std::ptr::copy_nonoverlapping(
                    (value.len() as u32).to_le_bytes().as_ptr(),
                    output.add(offset),
                    4,
                );
                std::ptr::copy_nonoverlapping(value.as_ptr(), output.add(offset + 4), value.len());
            }
            offset += 4 + value.len();
        }
        Ok(())
    });
    if status == 0 && insufficient {
        -3
    } else {
        status
    }
}
