// ===========================================================================
// serializer_wasm_legacy.inc -- residual ValueDeserializer legacy controls.
//
// Include after buffers_serialization.inc and serialization_delegates.inc so
// both wrapper families are complete. The family selector is intentionally
// private to the Go ABI: 0 is the base deserializer, 1 is the delegate-backed
// deserializer. These exports add no ownership; each wrapper retains its
// existing isolate, input-buffer and destruction rules.
// ===========================================================================

extern "C" {

__declspec(dllexport) int64_t gov8_swlr_deserializer_set_legacy(
    void* deserw, int32_t family, int32_t enabled) {
  ClearErr();
  if (enabled != 0 && enabled != 1) {
    SetErr("legacy support flag must be 0 or 1");
    return kErrBadArg;
  }
  try {
    if (family == 0) {
      gov8_buf_impl::DeserWrap* dw = gov8_buf_impl::AsDeser(deserw);
      if (dw == nullptr || dw->deser == nullptr) {
        SetErr("invalid base deserializer handle");
        return kErrMagic;
      }
      Gov8IsolateScope iso_scope(dw->iso);
      dw->deser->SetSupportsLegacyWireFormat(enabled != 0);
      return kOk;
    }
    if (family == 1) {
      gov8_serdel_impl::DeserDelWrap* dw =
          gov8_serdel_impl::AsDeserDel(deserw);
      if (dw == nullptr || dw->deser == nullptr) {
        SetErr("invalid delegate deserializer handle");
        return kErrMagic;
      }
      Gov8IsolateScope iso_scope(dw->iso);
      dw->deser->SetSupportsLegacyWireFormat(enabled != 0);
      return kOk;
    }
    SetErr("invalid deserializer family");
    return kErrBadArg;
  } catch (...) {
    SetErr("C++ exception in SetSupportsLegacyWireFormat");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_swlr_deserializer_wire_version(
    void* deserw, int32_t family, uint32_t* out) {
  ClearErr();
  if (out == nullptr) {
    SetErr("null wire version output");
    return kErrBadArg;
  }
  *out = 0;
  try {
    if (family == 0) {
      gov8_buf_impl::DeserWrap* dw = gov8_buf_impl::AsDeser(deserw);
      if (dw == nullptr || dw->deser == nullptr) {
        SetErr("invalid base deserializer handle");
        return kErrMagic;
      }
      Gov8IsolateScope iso_scope(dw->iso);
      *out = dw->deser->GetWireFormatVersion();
      return kOk;
    }
    if (family == 1) {
      gov8_serdel_impl::DeserDelWrap* dw =
          gov8_serdel_impl::AsDeserDel(deserw);
      if (dw == nullptr || dw->deser == nullptr) {
        SetErr("invalid delegate deserializer handle");
        return kErrMagic;
      }
      Gov8IsolateScope iso_scope(dw->iso);
      *out = dw->deser->GetWireFormatVersion();
      return kOk;
    }
    SetErr("invalid deserializer family");
    return kErrBadArg;
  } catch (...) {
    SetErr("C++ exception in GetWireFormatVersion");
    return kErrCpp;
  }
}

}  // extern "C"
