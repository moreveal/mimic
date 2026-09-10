// Residual public Symbol helpers from rusty_v8 152.2.0's symbol.rs.
//
// This file is intentionally self-contained so the coordinated shim include
// and ABI bump can land independently. It must be included after
// runtime_values.inc because it reuses that slice's catch macro.

extern "C" {
const v8::Symbol* v8__Symbol__GetAsyncIterator(v8::Isolate* isolate);
const v8::Symbol* v8__Symbol__GetIsConcatSpreadable(v8::Isolate* isolate);
const v8::Symbol* v8__Symbol__GetMatch(v8::Isolate* isolate);
const v8::Symbol* v8__Symbol__GetReplace(v8::Isolate* isolate);
const v8::Symbol* v8__Symbol__GetSearch(v8::Isolate* isolate);
const v8::Symbol* v8__Symbol__GetSplit(v8::Isolate* isolate);
const v8::Symbol* v8__Symbol__GetToPrimitive(v8::Isolate* isolate);
const v8::Symbol* v8__Symbol__GetUnscopables(v8::Isolate* isolate);
}

extern "C" {

#define GOV8_RVR_SYMBOL_WELLKNOWN(name, artifact)                              \
  __declspec(dllexport) int64_t name(v8::Isolate* iso, void* scope,            \
                                     void** out) {                             \
    ClearErr();                                                                \
    if (out == nullptr || !ScopeIs(iso, scope)) {                              \
      SetErr("invalid argument");                                              \
      return kErrBadArg;                                                       \
    }                                                                          \
    try {                                                                      \
      Gov8IsolateScope iso_scope(iso);                                         \
      *out = const_cast<void*>(static_cast<const void*>(artifact(iso)));       \
      return *out == nullptr ? kErrGeneric : kOk;                              \
    } GOV8_RV_END_CATCH(#name)                                                 \
  }

GOV8_RVR_SYMBOL_WELLKNOWN(gov8_rvr_symbol_get_async_iterator,
                          v8__Symbol__GetAsyncIterator)
GOV8_RVR_SYMBOL_WELLKNOWN(gov8_rvr_symbol_get_is_concat_spreadable,
                          v8__Symbol__GetIsConcatSpreadable)
GOV8_RVR_SYMBOL_WELLKNOWN(gov8_rvr_symbol_get_match, v8__Symbol__GetMatch)
GOV8_RVR_SYMBOL_WELLKNOWN(gov8_rvr_symbol_get_replace, v8__Symbol__GetReplace)
GOV8_RVR_SYMBOL_WELLKNOWN(gov8_rvr_symbol_get_search, v8__Symbol__GetSearch)
GOV8_RVR_SYMBOL_WELLKNOWN(gov8_rvr_symbol_get_split, v8__Symbol__GetSplit)
GOV8_RVR_SYMBOL_WELLKNOWN(gov8_rvr_symbol_get_to_primitive,
                          v8__Symbol__GetToPrimitive)
GOV8_RVR_SYMBOL_WELLKNOWN(gov8_rvr_symbol_get_unscopables,
                          v8__Symbol__GetUnscopables)

#undef GOV8_RVR_SYMBOL_WELLKNOWN

}  // extern "C"
