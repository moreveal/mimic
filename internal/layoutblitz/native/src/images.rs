use crate::ffi::{Handle, call};

#[unsafe(no_mangle)]
pub unsafe extern "C" fn mimic_blitz_image(
    handle: *mut Handle,
    id: u64,
    width: u32,
    height: u32,
    complete: u32,
) -> i32 {
    call(handle, |owner| {
        if complete > 1 {
            return Err(());
        }
        let node = owner.node(id).map_err(|_| ())?;
        if owner
            .document
            .set_canonical_image(node, width, height, complete == 1)?
        {
            owner.dirty = true;
        }
        Ok(())
    })
}
