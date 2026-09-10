// Custom PlatformImpl bridge for rusty_v8 152.2.0. The artifact's
// CustomPlatform owns PCContext; it transfers each foreground task pointer to
// Go, which must run-or-delete it exactly once.

#include <atomic>
#include <cstdint>
#include <new>

namespace gov8_pc_impl {

enum Kind : int32_t {
  kTask = 0,
  kNonNestable = 1,
  kDelayed = 2,
  kNonNestableDelayed = 3,
  kIdle = 4,
  kDrop = 5,
};

struct Context {
  uint64_t id;
};

Context* g_active_context = nullptr;

std::atomic<uint64_t> g_benchmark_created{0};
std::atomic<uint64_t> g_benchmark_run{0};
std::atomic<uint64_t> g_benchmark_destroyed{0};

class BenchmarkNoopTask final : public v8::Task {
 public:
  BenchmarkNoopTask() { g_benchmark_created.fetch_add(1); }
  ~BenchmarkNoopTask() override { g_benchmark_destroyed.fetch_add(1); }
  void Run() override { g_benchmark_run.fetch_add(1); }
};

struct Frame {
  int32_t kind;
  int32_t pad;
  uint64_t id;
  void* isolate;
  void* task;
  double delay;
};

using Dispatcher = uintptr_t (*)(Frame*);
Dispatcher g_dispatcher = nullptr;

void Dispatch(Kind kind, Context* context, void* isolate, void* task,
              double delay) {
  if (g_dispatcher == nullptr || context == nullptr) {
    std::abort();
  }
  Frame frame{static_cast<int32_t>(kind), 0, context->id, isolate, task,
              delay};
  g_dispatcher(&frame);
}

}  // namespace gov8_pc_impl

extern "C" {

v8::Platform* v8__Platform__NewCustomPlatform(int thread_pool_size,
                                               bool idle_task_support,
                                               bool unprotected,
                                               void* context);
void v8__Task__Run(v8::Task* task);
void v8__Task__DELETE(v8::Task* task);
void v8__IdleTask__Run(v8::IdleTask* task, double deadline_in_seconds);
void v8__IdleTask__DELETE(v8::IdleTask* task);

// These exact names satisfy the callbacks referenced by the pinned artifact's
// CustomPlatform and CustomTaskRunner implementation.
void v8__Platform__CustomPlatform__BASE__PostTask(void* context, void* isolate,
                                                  v8::Task* task) {
  gov8_pc_impl::Dispatch(gov8_pc_impl::kTask,
                         static_cast<gov8_pc_impl::Context*>(context), isolate,
                         task, 0.0);
}

void v8__Platform__CustomPlatform__BASE__PostNonNestableTask(
    void* context, void* isolate, v8::Task* task) {
  gov8_pc_impl::Dispatch(gov8_pc_impl::kNonNestable,
                         static_cast<gov8_pc_impl::Context*>(context), isolate,
                         task, 0.0);
}

void v8__Platform__CustomPlatform__BASE__PostDelayedTask(
    void* context, void* isolate, v8::Task* task, double delay) {
  gov8_pc_impl::Dispatch(gov8_pc_impl::kDelayed,
                         static_cast<gov8_pc_impl::Context*>(context), isolate,
                         task, delay);
}

void v8__Platform__CustomPlatform__BASE__PostNonNestableDelayedTask(
    void* context, void* isolate, v8::Task* task, double delay) {
  gov8_pc_impl::Dispatch(
      gov8_pc_impl::kNonNestableDelayed,
      static_cast<gov8_pc_impl::Context*>(context), isolate, task, delay);
}

void v8__Platform__CustomPlatform__BASE__PostIdleTask(void* context,
                                                      void* isolate,
                                                      v8::IdleTask* task) {
  gov8_pc_impl::Dispatch(gov8_pc_impl::kIdle,
                         static_cast<gov8_pc_impl::Context*>(context), isolate,
                         task, 0.0);
}

void v8__Platform__CustomPlatform__BASE__DROP(void* context) {
  auto* typed = static_cast<gov8_pc_impl::Context*>(context);
  if (gov8_pc_impl::g_active_context == typed) {
    gov8_pc_impl::g_active_context = nullptr;
  }
  gov8_pc_impl::Dispatch(gov8_pc_impl::kDrop, typed, nullptr, nullptr, 0.0);
  delete typed;
}

__declspec(dllexport) int64_t gov8_pc_set_dispatcher(uintptr_t dispatcher) {
  ClearErr();
  if (dispatcher == 0 || gov8_pc_impl::g_dispatcher != nullptr) {
    SetErr("invalid custom platform dispatcher");
    return kErrBadArg;
  }
  gov8_pc_impl::g_dispatcher =
      reinterpret_cast<gov8_pc_impl::Dispatcher>(dispatcher);
  return kOk;
}

__declspec(dllexport) int64_t gov8_pc_initialize_custom(
    int32_t thread_pool_size, int32_t idle_task_support, int32_t unprotected,
    uint64_t id) {
  ClearErr();
  if (g_platform != nullptr || gov8_pc_impl::g_dispatcher == nullptr ||
      thread_pool_size < 0 || thread_pool_size > 16 ||
      (idle_task_support != 0 && idle_task_support != 1) ||
      (unprotected != 0 && unprotected != 1) || id == 0) {
    SetErr("invalid custom platform configuration");
    return kErrBadArg;
  }
  auto* context = new (std::nothrow) gov8_pc_impl::Context{id};
  if (context == nullptr) {
    SetErr("custom platform context allocation failed");
    return kErrNoMemory;
  }
  bool platform_owns_context = false;
  try {
    v8::Platform* platform = v8__Platform__NewCustomPlatform(
        thread_pool_size, idle_task_support != 0, unprotected != 0, context);
    if (platform == nullptr) {
      delete context;
      SetErr("custom platform constructor failed");
      return kErrState;
    }
    platform_owns_context = true;
    v8::V8::InitializePlatform(platform);
    g_platform = platform;
    if (!v8::V8::Initialize()) {
      SetErr("V8::Initialize failed");
      return kErrState;
    }
    gov8_pc_impl::g_active_context = context;
    g_allocator.reset(v8::ArrayBuffer::Allocator::NewDefaultAllocator());
    return kOk;
  } catch (...) {
    if (!platform_owns_context) delete context;
    SetErr("C++ exception initializing custom platform");
    return kErrCpp;
  }
}

// Benchmark-only synthetic task producer. It drives the same CustomPlatform
// PostTask callback and one-shot Task::Run/delete ownership path as a real V8
// foreground task without adding unrelated compiler or Wasm work.
__declspec(dllexport) int64_t gov8_pc_benchmark_post_noop_task(void) {
  ClearErr();
  auto* context = gov8_pc_impl::g_active_context;
  v8::Isolate* isolate = v8::Isolate::GetCurrent();
  if (context == nullptr || isolate == nullptr) {
    SetErr("custom platform benchmark requires an active context and isolate");
    return kErrState;
  }
  auto* task = new (std::nothrow) gov8_pc_impl::BenchmarkNoopTask();
  if (task == nullptr) {
    SetErr("custom platform benchmark task allocation failed");
    return kErrNoMemory;
  }
  v8__Platform__CustomPlatform__BASE__PostTask(context, isolate, task);
  return kOk;
}

__declspec(dllexport) int64_t gov8_pc_benchmark_reset_noop_task_counts(void) {
  ClearErr();
  gov8_pc_impl::g_benchmark_created.store(0);
  gov8_pc_impl::g_benchmark_run.store(0);
  gov8_pc_impl::g_benchmark_destroyed.store(0);
  return kOk;
}

__declspec(dllexport) int64_t gov8_pc_benchmark_noop_task_counts(
    uint64_t* created, uint64_t* run, uint64_t* destroyed) {
  ClearErr();
  if (created == nullptr || run == nullptr || destroyed == nullptr) {
    SetErr("null custom platform benchmark counter output");
    return kErrBadArg;
  }
  *created = gov8_pc_impl::g_benchmark_created.load();
  *run = gov8_pc_impl::g_benchmark_run.load();
  *destroyed = gov8_pc_impl::g_benchmark_destroyed.load();
  return kOk;
}

__declspec(dllexport) int64_t gov8_pc_task_run_delete(v8::Task* task) {
  ClearErr();
  if (task == nullptr) {
    SetErr("null Task");
    return kErrBadArg;
  }
  try {
    v8__Task__Run(task);
    v8__Task__DELETE(task);
    return kOk;
  } catch (...) {
    v8__Task__DELETE(task);
    SetErr("C++ exception running Task");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_pc_task_delete(v8::Task* task) {
  ClearErr();
  if (task == nullptr) {
    SetErr("null Task");
    return kErrBadArg;
  }
  v8__Task__DELETE(task);
  return kOk;
}

__declspec(dllexport) int64_t gov8_pc_idle_task_run_delete(
    v8::IdleTask* task, double deadline) {
  ClearErr();
  if (task == nullptr) {
    SetErr("null IdleTask");
    return kErrBadArg;
  }
  try {
    v8__IdleTask__Run(task, deadline);
    v8__IdleTask__DELETE(task);
    return kOk;
  } catch (...) {
    v8__IdleTask__DELETE(task);
    SetErr("C++ exception running IdleTask");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_pc_idle_task_delete(v8::IdleTask* task) {
  ClearErr();
  if (task == nullptr) {
    SetErr("null IdleTask");
    return kErrBadArg;
  }
  v8__IdleTask__DELETE(task);
  return kOk;
}

}  // extern "C"
