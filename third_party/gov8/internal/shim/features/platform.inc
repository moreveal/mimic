// Default-platform selection and idle-task execution for rusty_v8 152.2.0.

extern "C" {
v8::Platform* v8__Platform__NewUnprotectedDefaultPlatform(
    int thread_pool_size, bool idle_task_support);
v8::Platform* v8__Platform__NewSingleThreadedDefaultPlatform(
    bool idle_task_support);
void v8__Platform__RunIdleTasks(v8::Platform* platform,
                                v8::Isolate* isolate,
                                double idle_time_in_seconds);
}

extern "C" {

__declspec(dllexport) int64_t gov8_platform_initialize_configured(
    int32_t kind, int32_t thread_pool_size, int32_t idle_task_support) {
  ClearErr();
  if (kind < 0 || kind > 2 || thread_pool_size < 0 || thread_pool_size > 16 ||
      (idle_task_support != 0 && idle_task_support != 1)) {
    SetErr("invalid platform configuration");
    return kErrBadArg;
  }
  if (kind == 2 && thread_pool_size != 0) {
    SetErr("single-threaded platform has no worker pool");
    return kErrBadArg;
  }
  try {
    if (g_platform != nullptr) {
      SetErr("platform already created");
      return kErrState;
    }
    v8::Platform* platform = nullptr;
    switch (kind) {
      case 0:
        platform = v8__Platform__NewDefaultPlatform(thread_pool_size,
                                                     idle_task_support != 0);
        break;
      case 1:
        platform = v8__Platform__NewUnprotectedDefaultPlatform(
            thread_pool_size, idle_task_support != 0);
        break;
      case 2:
        platform = v8__Platform__NewSingleThreadedDefaultPlatform(
            idle_task_support != 0);
        break;
    }
    if (platform == nullptr) {
      SetErr("platform constructor failed");
      return kErrState;
    }
    v8::V8::InitializePlatform(platform);
    g_platform = platform;
    if (!v8::V8::Initialize()) {
      SetErr("V8::Initialize failed");
      return kErrState;
    }
    g_allocator.reset(v8::ArrayBuffer::Allocator::NewDefaultAllocator());
    return kOk;
  } catch (...) {
    SetErr("C++ exception initializing configured platform");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_platform_run_idle_tasks(
    v8::Isolate* isolate, double idle_time_in_seconds) {
  ClearErr();
  if (g_platform == nullptr || isolate == nullptr) {
    SetErr("platform or isolate is not available");
    return kErrState;
  }
  try {
    Gov8IsolateScope isolate_scope(isolate);
    v8__Platform__RunIdleTasks(g_platform, isolate, idle_time_in_seconds);
    return kOk;
  } catch (...) {
    SetErr("C++ exception running idle tasks");
    return kErrCpp;
  }
}

}  // extern "C"
