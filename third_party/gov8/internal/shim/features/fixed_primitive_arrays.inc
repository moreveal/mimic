// FixedArray and PrimitiveArray local-handle operations. All index and length
// checks are repeated here as a defensive boundary even though the Go API
// validates them first; unchecked V8 PrimitiveArray access is process-fatal.

extern "C" {

__declspec(dllexport) int64_t gov8_data_is_value(v8::Isolate* iso,
                                                 void* data_wire) {
  ClearErr();
  if (iso == nullptr || data_wire == nullptr) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    return FromWire<v8::Data>(data_wire)->IsValue() ? 1 : 0;
  } catch (...) {
    SetErr("C++ exception in data_is_value");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_data_is_primitive(v8::Isolate* iso,
                                                     void* data_wire) {
  ClearErr();
  if (iso == nullptr || data_wire == nullptr) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    v8::Local<v8::Data> data = FromWire<v8::Data>(data_wire);
    return data->IsValue() && data.As<v8::Value>()->IsPrimitive() ? 1 : 0;
  } catch (...) {
    SetErr("C++ exception in data_is_primitive");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_data_is_fixed_array(v8::Isolate* iso,
                                                       void* data_wire) {
  ClearErr();
  if (iso == nullptr || data_wire == nullptr) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    return FromWire<v8::Data>(data_wire)->IsFixedArray() ? 1 : 0;
  } catch (...) {
    SetErr("C++ exception in data_is_fixed_array");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_data_is_module_request(v8::Isolate* iso,
                                                          void* data_wire) {
  ClearErr();
  if (iso == nullptr || data_wire == nullptr) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    return FromWire<v8::Data>(data_wire)->IsModuleRequest() ? 1 : 0;
  } catch (...) {
    SetErr("C++ exception in data_is_module_request");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_value_is_primitive(v8::Isolate* iso,
                                                      void* value_wire) {
  ClearErr();
  if (iso == nullptr || value_wire == nullptr) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    return FromWire<v8::Value>(value_wire)->IsPrimitive() ? 1 : 0;
  } catch (...) {
    SetErr("C++ exception in value_is_primitive");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_primitive_array_new(
    v8::Isolate* iso, void* scope, int64_t length, void** out) {
  ClearErr();
  if (out == nullptr || !ScopeIs(iso, scope) || length < 0 ||
      length > INT32_MAX) {
    SetErr("primitive array length outside C int range");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    *out = ToWire(v8::PrimitiveArray::New(iso, static_cast<int>(length)));
    return kOk;
  } catch (...) {
    SetErr("C++ exception in primitive_array_new");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_primitive_array_length(
    v8::Isolate* iso, void* array_wire) {
  ClearErr();
  if (iso == nullptr || array_wire == nullptr) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    return FromWire<v8::PrimitiveArray>(array_wire)->Length();
  } catch (...) {
    SetErr("C++ exception in primitive_array_length");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_primitive_array_get(
    v8::Isolate* iso, void* scope, void* array_wire, int64_t index,
    void** out) {
  ClearErr();
  if (out == nullptr || array_wire == nullptr || !ScopeIs(iso, scope)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    v8::Local<v8::PrimitiveArray> array =
        FromWire<v8::PrimitiveArray>(array_wire);
    if (index < 0 || index >= array->Length()) {
      SetErr("primitive array index out of range");
      return kErrBadArg;
    }
    *out = ToWire(array->Get(iso, static_cast<int>(index)));
    return kOk;
  } catch (...) {
    SetErr("C++ exception in primitive_array_get");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_primitive_array_set(
    v8::Isolate* iso, void* scope, void* array_wire, int64_t index,
    void* item_wire) {
  ClearErr();
  if (array_wire == nullptr || item_wire == nullptr || !ScopeIs(iso, scope)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    v8::Local<v8::PrimitiveArray> array =
        FromWire<v8::PrimitiveArray>(array_wire);
    v8::Local<v8::Value> item = FromWire<v8::Value>(item_wire);
    if (index < 0 || index >= array->Length()) {
      SetErr("primitive array index out of range");
      return kErrBadArg;
    }
    if (!item->IsPrimitive()) {
      SetErr("primitive array item is not primitive");
      return kErrBadArg;
    }
    array->Set(iso, static_cast<int>(index), item.As<v8::Primitive>());
    return kOk;
  } catch (...) {
    SetErr("C++ exception in primitive_array_set");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_fixed_array_length(v8::Isolate* iso,
                                                      void* array_wire) {
  ClearErr();
  if (iso == nullptr || array_wire == nullptr) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    return FromWire<v8::FixedArray>(array_wire)->Length();
  } catch (...) {
    SetErr("C++ exception in fixed_array_length");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_fixed_array_get(
    v8::Isolate* iso, void* scope, void* array_wire, int64_t index,
    void** out) {
  ClearErr();
  if (out == nullptr || array_wire == nullptr || !ScopeIs(iso, scope)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    v8::Local<v8::FixedArray> array = FromWire<v8::FixedArray>(array_wire);
    if (index < 0 || index >= array->Length()) {
      return 1;
    }
    *out = ToWire(array->Get(static_cast<int>(index)));
    return kOk;
  } catch (...) {
    SetErr("C++ exception in fixed_array_get");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_module_requests_fixed_array(
    v8::Isolate* iso, void* scope, void* module_wire, void** out) {
  ClearErr();
  ModuleWrap* module = AsModule(module_wire);
  if (out == nullptr || module == nullptr || !ScopeIs(iso, scope) ||
      module->iso != iso) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    *out = ToWire(module->module->Get(iso)->GetModuleRequests());
    return kOk;
  } catch (...) {
    SetErr("C++ exception in module_requests_fixed_array");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_module_request_import_attributes(
    v8::Isolate* iso, void* scope, void* request_wire, void** out) {
  ClearErr();
  if (out == nullptr || request_wire == nullptr || !ScopeIs(iso, scope)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    v8::Local<v8::Data> data = FromWire<v8::Data>(request_wire);
    if (!data->IsModuleRequest()) {
      SetErr("data is not a module request");
      return kErrBadArg;
    }
    *out = ToWire(data.As<v8::ModuleRequest>()->GetImportAttributes());
    return kOk;
  } catch (...) {
    SetErr("C++ exception in module_request_import_attributes");
    return kErrCpp;
  }
}

}  // extern "C"
