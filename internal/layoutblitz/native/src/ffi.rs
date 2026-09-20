//! ABI calls are serialized by the owning Page. No Go pointers are retained.
use crate::{Owner, Rect};
use std::panic::{AssertUnwindSafe, catch_unwind};

pub struct Handle {
    owner: Owner,
    poisoned: bool,
}

unsafe fn text<'a>(ptr: *const u8, len: usize) -> Result<&'a str, ()> {
    if len == 0 {
        return Ok("");
    }
    if ptr.is_null() {
        return Err(());
    }
    std::str::from_utf8(unsafe { std::slice::from_raw_parts(ptr, len) }).map_err(|_| ())
}

fn call(handle: *mut Handle, f: impl FnOnce(&mut Owner) -> Result<(), ()>) -> i32 {
    if handle.is_null() {
        return -1;
    }
    let handle = unsafe { &mut *handle };
    if handle.poisoned {
        return -2;
    }
    match catch_unwind(AssertUnwindSafe(|| f(&mut handle.owner))) {
        Ok(Ok(())) => 0,
        _ => {
            handle.poisoned = true;
            -2
        }
    }
}

#[unsafe(no_mangle)]
pub extern "C" fn mimic_blitz_new(root: u64, width: u32, height: u32) -> *mut Handle {
    catch_unwind(AssertUnwindSafe(|| {
        Box::into_raw(Box::new(Handle {
            owner: Owner::new(root, width, height),
            poisoned: false,
        }))
    }))
    .unwrap_or(std::ptr::null_mut())
}

#[unsafe(no_mangle)]
pub unsafe extern "C" fn mimic_blitz_drop(handle: *mut Handle) {
    if !handle.is_null() {
        drop(unsafe { Box::from_raw(handle) });
    }
}

#[unsafe(no_mangle)]
pub unsafe extern "C" fn mimic_blitz_element(
    handle: *mut Handle,
    id: u64,
    ns: *const u8,
    ns_len: usize,
    name: *const u8,
    name_len: usize,
) -> i32 {
    call(handle, |owner| {
        owner
            .element(id, unsafe { text(ns, ns_len)? }, unsafe {
                text(name, name_len)?
            })
            .map_err(|_| ())
    })
}

#[unsafe(no_mangle)]
pub unsafe extern "C" fn mimic_blitz_attribute(
    handle: *mut Handle,
    id: u64,
    ns: *const u8,
    ns_len: usize,
    name: *const u8,
    name_len: usize,
    value: *const u8,
    value_len: usize,
) -> i32 {
    call(handle, |owner| {
        owner
            .attribute(
                id,
                unsafe { text(ns, ns_len)? },
                unsafe { text(name, name_len)? },
                unsafe { text(value, value_len)? },
            )
            .map_err(|_| ())
    })
}

#[unsafe(no_mangle)]
pub extern "C" fn mimic_blitz_append(handle: *mut Handle, parent: u64, child: u64) -> i32 {
    call(handle, |owner| owner.append(parent, child).map_err(|_| ()))
}

#[unsafe(no_mangle)]
pub unsafe extern "C" fn mimic_blitz_text(
    handle: *mut Handle,
    id: u64,
    value: *const u8,
    len: usize,
    create: u32,
) -> i32 {
    call(handle, |owner| {
        let value = unsafe { text(value, len)? };
        if create != 0 {
            owner.text(id, value)
        } else {
            owner.set_text(id, value)
        }
        .map_err(|_| ())
    })
}

#[unsafe(no_mangle)]
pub unsafe extern "C" fn mimic_blitz_resolve(
    handle: *mut Handle,
    time: f64,
    generation: *mut u64,
) -> i32 {
    if generation.is_null() {
        return -1;
    }
    call(handle, |owner| {
        owner.resolve(time);
        unsafe {
            *generation = owner.generation;
        }
        Ok(())
    })
}

#[unsafe(no_mangle)]
pub unsafe extern "C" fn mimic_blitz_rect(handle: *mut Handle, id: u64, output: *mut Rect) -> i32 {
    if output.is_null() {
        return -1;
    }
    call(handle, |owner| {
        let rect = owner.rect(id).map_err(|_| ())?;
        unsafe {
            *output = rect;
        }
        Ok(())
    })
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn invalid_update_poisons_owner_until_disposal() {
        let handle = mimic_blitz_new(1, 800, 600);
        assert!(!handle.is_null());
        assert_eq!(mimic_blitz_append(handle, 1, 999), -2);
        let mut generation = 0;
        assert_eq!(
            unsafe { mimic_blitz_resolve(handle, 0.0, &mut generation) },
            -2
        );
        unsafe {
            mimic_blitz_drop(handle);
        }
    }
}
