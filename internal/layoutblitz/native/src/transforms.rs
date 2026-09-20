use crate::ffi::{Handle, call};

#[unsafe(no_mangle)]
pub unsafe extern "C" fn mimic_blitz_inline_declarations(
    handle: *mut Handle,
    id: u64,
    data: *const u8,
    len: usize,
) -> i32 {
    call(handle, |owner| {
        let css = if len == 0 {
            ""
        } else {
            if data.is_null() {
                return Err(());
            }
            std::str::from_utf8(unsafe { std::slice::from_raw_parts(data, len) }).map_err(|_| ())?
        };
        let node = owner.node(id).map_err(|_| ())?;
        owner.document.set_canonical_inline_declarations(node, css);
        owner.dirty = true;
        Ok(())
    })
}

#[unsafe(no_mangle)]
pub unsafe extern "C" fn mimic_blitz_transform(
    handle: *mut Handle,
    id: u64,
    output: *mut f64,
    present: *mut u32,
) -> i32 {
    call(handle, |owner| {
        if owner.dirty || output.is_null() || present.is_null() {
            return Err(());
        }
        let node = owner.node(id).map_err(|_| ())?;
        if let Some(values) = owner.document.geometry_transform(node) {
            unsafe {
                std::ptr::copy_nonoverlapping(values.as_ptr(), output, 19);
                *present = 1;
            }
        } else {
            unsafe {
                *present = 0;
            }
        }
        Ok(())
    })
}
