// Safe cppgc process and custom-heap lifecycle for pinned V8 15.2.

#include <mutex>
#include <unordered_set>

extern "C" {
void cppgc__initialize_process(v8::Platform* platform);
v8::Platform* v8__Platform__NewUnprotectedDefaultPlatform(
    int thread_pool_size, bool idle_task_support);
v8::CppHeap* v8__CppHeap__Create(v8::Platform* platform,
                                 uint8_t marking, uint8_t sweeping);
void v8__CppHeap__Terminate(v8::CppHeap* heap);
void v8__CppHeap__DELETE(v8::CppHeap* heap);
void cppgc__heap__enable_detached_garbage_collections_for_testing(
    v8::CppHeap* heap);
void cppgc__heap__collect_garbage_for_testing(
    v8::CppHeap* heap, cppgc::EmbedderStackState stack_state);
}

namespace gov8_cppgc_heap_impl {

constexpr uint64_t kHeapMagic = 0x674f563843504850ULL;
struct HeapWrap {
  uint64_t magic = kHeapMagic;
  v8::CppHeap* heap;
  bool terminated = false;
  bool detached_enabled = false;
  explicit HeapWrap(v8::CppHeap* value) : heap(value) {}
};

std::mutex heap_mutex;
std::unordered_set<HeapWrap*> heaps;
v8::Platform* detached_platform = nullptr;
bool detached_initialized = false;

HeapWrap* Checked(void* wire) {
  auto* wrap = static_cast<HeapWrap*>(wire);
  if (wrap == nullptr || wrap->magic != kHeapMagic || wrap->heap == nullptr)
    return nullptr;
  std::lock_guard<std::mutex> lock(heap_mutex);
  return heaps.count(wrap) == 1 ? wrap : nullptr;
}

void Forget(HeapWrap* wrap) {
  std::lock_guard<std::mutex> lock(heap_mutex);
  heaps.erase(wrap);
}

bool TakeForParams(void* wire, v8::CppHeap** heap, void** identity) noexcept {
  HeapWrap* wrap = Checked(wire);
  if (wrap == nullptr || wrap->terminated || wrap->detached_enabled ||
      heap == nullptr || identity == nullptr)
    return false;
  *identity = wrap->heap;
  *heap = wrap->heap;
  wrap->heap = nullptr;
  wrap->magic = 0;
  Forget(wrap);
  delete wrap;
  return true;
}

}  // namespace gov8_cppgc_heap_impl

extern "C" {

__declspec(dllexport) int64_t gov8_cppgc_process_initialize(void) {
  using namespace gov8_cppgc_heap_impl;
  ClearErr();
  if (detached_initialized || g_platform != nullptr) {
    SetErr("cppgc process already initialized or V8 active");
    return kErrState;
  }
  try {
    if (detached_platform == nullptr) {
      detached_platform = v8__Platform__NewUnprotectedDefaultPlatform(0, 0);
      if (detached_platform == nullptr) {
        SetErr("cppgc platform creation failed");
        return kErrState;
      }
    }
    cppgc__initialize_process(detached_platform);
    detached_initialized = true;
    return kOk;
  } catch (...) {
    SetErr("C++ exception initializing cppgc process");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_cppgc_process_shutdown(void) {
  using namespace gov8_cppgc_heap_impl;
  ClearErr();
  if (!detached_initialized || detached_platform == nullptr) {
    SetErr("cppgc process is not explicitly initialized");
    return kErrState;
  }
  {
    std::lock_guard<std::mutex> lock(heap_mutex);
    if (!heaps.empty()) {
      SetErr("cppgc process has live heaps");
      return kErrState;
    }
  }
  try {
    cppgc__shutdown_process();
    detached_initialized = false;
    return kOk;
  } catch (...) {
    SetErr("C++ exception shutting down cppgc process");
    return kErrCpp;
  }
}

__declspec(dllexport) void* gov8_cppgc_heap_create(int32_t marking,
                                                   int32_t sweeping) {
  using namespace gov8_cppgc_heap_impl;
  ClearErr();
  if (marking < 0 || marking > 2 || sweeping < 0 || sweeping > 2) {
    SetErr("invalid cppgc heap support enum");
    return nullptr;
  }
  v8::Platform* platform = detached_initialized ? detached_platform
                                                         : g_platform;
  if (platform == nullptr) {
    SetErr("cppgc process is not initialized");
    return nullptr;
  }
  try {
    v8::CppHeap* heap = v8__CppHeap__Create(
        platform, static_cast<uint8_t>(marking),
        static_cast<uint8_t>(sweeping));
    if (heap == nullptr) {
      SetErr("cppgc heap creation failed");
      return nullptr;
    }
    std::unique_ptr<HeapWrap> wrap(new HeapWrap(heap));
    {
      std::lock_guard<std::mutex> lock(heap_mutex);
      heaps.insert(wrap.get());
    }
    return wrap.release();
  } catch (...) {
    SetErr("C++ exception creating cppgc heap");
    return nullptr;
  }
}

__declspec(dllexport) int64_t gov8_cppgc_heap_enable_detached(void* wire) {
  using namespace gov8_cppgc_heap_impl;
  ClearErr();
  HeapWrap* wrap = Checked(wire);
  if (wrap == nullptr || wrap->terminated || wrap->detached_enabled) {
    SetErr("cppgc heap cannot enter detached testing mode");
    return kErrState;
  }
  try {
    cppgc__heap__enable_detached_garbage_collections_for_testing(wrap->heap);
    wrap->detached_enabled = true;
    return kOk;
  } catch (...) {
    SetErr("C++ exception enabling detached cppgc collection");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_cppgc_heap_collect(void* wire,
                                                      int32_t stack_state) {
  using namespace gov8_cppgc_heap_impl;
  ClearErr();
  HeapWrap* wrap = Checked(wire);
  if (wrap == nullptr || wrap->terminated || stack_state < 0 ||
      stack_state > 1) {
    SetErr("invalid cppgc heap collection argument");
    return kErrBadArg;
  }
  try {
    cppgc__heap__collect_garbage_for_testing(
        wrap->heap, static_cast<cppgc::EmbedderStackState>(stack_state));
    return kOk;
  } catch (...) {
    SetErr("C++ exception collecting cppgc heap");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_cppgc_heap_allocate_leaf(
    void* wire, uint64_t registry_id, int32_t object_id, void* dispatch,
    int32_t* consumed) {
  using namespace gov8_cppgc_heap_impl;
  using namespace gov8_cppgc_impl;
  ClearErr();
  if (consumed != nullptr) *consumed = 0;
  HeapWrap* wrap = Checked(wire);
  if (wrap == nullptr || wrap->terminated || registry_id == 0 ||
      dispatch == nullptr || consumed == nullptr) {
    SetErr("invalid cppgc leaf allocation argument");
    return kErrBadArg;
  }
  try {
    GoDispatcher supplied = reinterpret_cast<GoDispatcher>(dispatch);
    GoDispatcher installed = dispatcher.load(std::memory_order_acquire);
    if (installed != nullptr && installed != supplied) {
      SetErr("cppgc dispatcher changed");
      return kErrState;
    }
    dispatcher.store(supplied, std::memory_order_release);
    RustObj* object = cppgc__make_garbage_collectable(
        wrap->heap, kAdditionalBytes, kAllocationAlignment);
    if (object == nullptr) {
      SetErr("cppgc leaf allocation failed");
      return kErrNoMemory;
    }
    *consumed = 1;
    new (PayloadOf(object)) Payload(registry_id, object_id, 0, false);
    return kOk;
  } catch (...) {
    SetErr("C++ exception allocating cppgc leaf");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_cppgc_heap_terminate(void* wire) {
  using namespace gov8_cppgc_heap_impl;
  ClearErr();
  HeapWrap* wrap = Checked(wire);
  if (wrap == nullptr) {
    SetErr("invalid cppgc heap");
    return kErrBadArg;
  }
  if (wrap->terminated) return kOk;
  try {
    v8__CppHeap__Terminate(wrap->heap);
    wrap->terminated = true;
    return kOk;
  } catch (...) {
    SetErr("C++ exception terminating cppgc heap");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_cppgc_heap_close(void* wire) {
  using namespace gov8_cppgc_heap_impl;
  ClearErr();
  HeapWrap* wrap = Checked(wire);
  if (wrap == nullptr) {
    SetErr("invalid cppgc heap");
    return kErrBadArg;
  }
  try {
    if (!wrap->terminated) v8__CppHeap__Terminate(wrap->heap);
    v8__CppHeap__DELETE(wrap->heap);
    wrap->heap = nullptr;
    wrap->magic = 0;
    Forget(wrap);
    delete wrap;
    return kOk;
  } catch (...) {
    SetErr("C++ exception closing cppgc heap");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_cppgc_heap_matches_isolate(
    void* heap_identity, v8::Isolate* isolate, int32_t* matches) {
  ClearErr();
  if (heap_identity == nullptr || isolate == nullptr || matches == nullptr) {
    SetErr("invalid cppgc heap identity argument");
    return kErrBadArg;
  }
  *matches = isolate->GetCppHeap() == heap_identity ? 1 : 0;
  return kOk;
}

}  // extern "C"

extern "C" v8::Platform* gov8_cppgc_take_detached_platform() {
  using namespace gov8_cppgc_heap_impl;
  if (detached_initialized) return nullptr;
  v8::Platform* result = detached_platform;
  detached_platform = nullptr;
  return result;
}
