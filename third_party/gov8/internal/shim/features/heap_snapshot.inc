// Heap-snapshot streaming for pinned rusty_v8 152.2.0. The artifact owns
// snapshot creation, serialization, deletion, abort, and terminal-empty-chunk
// behavior; this adapter only substitutes a process-stable Go dispatcher and
// a numeric per-call registry ID.

#include <atomic>

extern "C" {
using Gov8HeapSnapshotCallback = bool (*)(void*, const char*, size_t);
void v8__HeapProfiler__TakeHeapSnapshot(v8::Isolate* isolate,
                                        Gov8HeapSnapshotCallback callback,
                                        void* arg);
}

namespace gov8_heap_snapshot_impl {

using GoDispatcher = uintptr_t (*)(uint64_t, const uint8_t*, size_t);
std::atomic<GoDispatcher> dispatcher{nullptr};

int64_t Fail(const char* message) {
  SetErr(message);
  return -1;
}

bool Dispatch(void* arg, const char* data, size_t size) {
  GoDispatcher callback = dispatcher.load(std::memory_order_acquire);
  if (callback == nullptr) std::abort();
  const uint64_t id = static_cast<uint64_t>(reinterpret_cast<uintptr_t>(arg));
  return callback(id, reinterpret_cast<const uint8_t*>(data), size) != 0;
}

}  // namespace gov8_heap_snapshot_impl

extern "C" {

__declspec(dllexport) int64_t gov8_heap_snapshot_set_dispatcher(
    uintptr_t dispatcher) {
  ClearErr();
  if (dispatcher == 0) {
    return gov8_heap_snapshot_impl::Fail("heap snapshot dispatcher is null");
  }
  gov8_heap_snapshot_impl::dispatcher.store(
      reinterpret_cast<gov8_heap_snapshot_impl::GoDispatcher>(dispatcher),
      std::memory_order_release);
  return 0;
}

__declspec(dllexport) int64_t gov8_heap_snapshot_take(v8::Isolate* isolate,
                                                       uint64_t id) {
  ClearErr();
  if (isolate == nullptr) {
    return gov8_heap_snapshot_impl::Fail("heap snapshot isolate is null");
  }
  if (id == 0) {
    return gov8_heap_snapshot_impl::Fail("heap snapshot registry ID is zero");
  }
  if (gov8_heap_snapshot_impl::dispatcher.load(std::memory_order_acquire) ==
      nullptr) {
    return gov8_heap_snapshot_impl::Fail(
        "heap snapshot dispatcher is not installed");
  }
  try {
    Gov8IsolateScope isolate_scope(isolate);
    v8__HeapProfiler__TakeHeapSnapshot(
        isolate, gov8_heap_snapshot_impl::Dispatch,
        reinterpret_cast<void*>(static_cast<uintptr_t>(id)));
    return 0;
  } catch (...) {
    return gov8_heap_snapshot_impl::Fail("heap snapshot creation failed");
  }
}

}  // extern "C"
