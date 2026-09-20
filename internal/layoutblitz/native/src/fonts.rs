//! One bounded transaction transfers the current canonical font collection.
use crate::ffi::{Handle, call};

fn take<'a>(input: &mut &'a [u8], count: usize) -> Result<&'a [u8], ()> {
    if count > input.len() {
        return Err(());
    }
    let (head, tail) = input.split_at(count);
    *input = tail;
    Ok(head)
}
fn integer(input: &mut &[u8]) -> Result<u32, ()> {
    Ok(u32::from_le_bytes(
        take(input, 4)?.try_into().map_err(|_| ())?,
    ))
}

#[unsafe(no_mangle)]
pub unsafe extern "C" fn mimic_blitz_fonts(
    handle: *mut Handle,
    bytes: *const u8,
    len: usize,
) -> i32 {
    call(handle, |owner| {
        if bytes.is_null() || len > 64 * 1024 * 1024 {
            return Err(());
        }
        let mut input = unsafe { std::slice::from_raw_parts(bytes, len) };
        let count = integer(&mut input)?;
        if count > 4096 {
            return Err(());
        }
        let mut fonts = Vec::with_capacity(count as usize);
        for _ in 0..count {
            let family_len = integer(&mut input)? as usize;
            let data_len = integer(&mut input)? as usize;
            let weight = f32::from_bits(integer(&mut input)?);
            let italic = integer(&mut input)?;
            if !weight.is_finite() || weight < 1.0 || weight > 1000.0 || italic > 1 {
                return Err(());
            }
            let family = std::str::from_utf8(take(&mut input, family_len)?)
                .map_err(|_| ())?
                .to_string();
            let data = take(&mut input, data_len)?.to_vec();
            fonts.push(blitz_dom::CanonicalFont {
                family,
                weight,
                italic: italic == 1,
                bytes: data,
            });
        }
        if !input.is_empty() {
            return Err(());
        }
        owner.document.replace_canonical_fonts(fonts);
        owner.dirty = true;
        Ok(())
    })
}
