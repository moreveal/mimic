// temporal-link is the staticlib root of the shim's temporal dependency
// closure: it brings std's global allocator and panic handler into the
// archive. The temporal_capi / temporal_rs object code itself is consumed
// from the dependency rlibs on the DLL link line (COFF archives; the MSVC
// linker pulls members on demand).
extern crate temporal_capi;
