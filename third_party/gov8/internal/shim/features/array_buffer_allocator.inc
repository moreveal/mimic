// Safe ArrayBuffer allocator callbacks and shared ownership.

#include <cstdlib>
#include <memory>

namespace gov8_array_buffer_allocator_impl {

using GoAllocatorDispatcher = uintptr_t (*)(uintptr_t, uintptr_t, size_t,
                                             uintptr_t);

constexpr uint64_t kAllocatorMagic = 0x674f56384142414cULL;
constexpr uintptr_t kAllocate = 1;
constexpr uintptr_t kAllocateUninitialized = 2;
constexpr uintptr_t kFree = 3;
constexpr uintptr_t kDrop = 4;

GoAllocatorDispatcher dispatcher = nullptr;

class HostAllocator final : public v8::ArrayBuffer::Allocator {
 public:
  explicit HostAllocator(uintptr_t id) : id_(id) {}
  HostAllocator(const HostAllocator&) = delete;
  HostAllocator& operator=(const HostAllocator&) = delete;

  ~HostAllocator() override {
    if (dispatcher != nullptr) dispatcher(id_, kDrop, 0, 0);
  }

  void* Allocate(size_t length) final {
    if (length == 0) return nullptr;
    if (dispatcher == nullptr || dispatcher(id_, kAllocate, length, 0) == 0)
      return nullptr;
    return std::calloc(1, length);
  }

  void* AllocateUninitialized(size_t length) final {
    if (length == 0) return nullptr;
    if (dispatcher == nullptr ||
        dispatcher(id_, kAllocateUninitialized, length, 0) == 0)
      return nullptr;
    return std::malloc(length);
  }

  void Free(void* data, size_t length) final {
    uintptr_t first = 0;
    if (data != nullptr && length != 0)
      first = static_cast<uintptr_t>(*static_cast<unsigned char*>(data));
    if (dispatcher != nullptr) dispatcher(id_, kFree, length, first);
    std::free(data);
  }

 private:
  uintptr_t id_;
};

struct AllocatorWrap {
  uint64_t magic = kAllocatorMagic;
  std::shared_ptr<v8::ArrayBuffer::Allocator> allocator;
  explicit AllocatorWrap(std::shared_ptr<v8::ArrayBuffer::Allocator> value)
      : allocator(std::move(value)) {}
};

AllocatorWrap* Checked(void* wire) {
  auto* wrap = static_cast<AllocatorWrap*>(wire);
  if (wrap == nullptr || wrap->magic != kAllocatorMagic || !wrap->allocator)
    return nullptr;
  return wrap;
}

void ReleaseRetainedAllocator(void* raw) noexcept {
  delete static_cast<std::shared_ptr<v8::ArrayBuffer::Allocator>*>(raw);
}

bool FillCreateParams(void* wire,
                      v8::Isolate::CreateParams* params) noexcept {
  auto* wrap = Checked(wire);
  if (wrap == nullptr || params == nullptr) return false;
  params->array_buffer_allocator_shared = wrap->allocator;
  params->array_buffer_allocator = nullptr;
  return true;
}

}  // namespace gov8_array_buffer_allocator_impl

extern "C" {

__declspec(dllexport) int64_t gov8_aba_set_dispatcher(void* raw_dispatcher) {
  ClearErr();
  if (raw_dispatcher == nullptr) {
    SetErr("null ArrayBuffer allocator dispatcher");
    return kErrBadArg;
  }
  gov8_array_buffer_allocator_impl::dispatcher =
      reinterpret_cast<gov8_array_buffer_allocator_impl::GoAllocatorDispatcher>(
          raw_dispatcher);
  return kOk;
}

__declspec(dllexport) void* gov8_aba_new_default() {
  ClearErr();
  try {
    std::shared_ptr<v8::ArrayBuffer::Allocator> allocator(
        v8::ArrayBuffer::Allocator::NewDefaultAllocator());
    if (!allocator) {
      SetErr("NewDefaultAllocator failed");
      return nullptr;
    }
    return new gov8_array_buffer_allocator_impl::AllocatorWrap(
        std::move(allocator));
  } catch (...) {
    SetErr("C++ exception creating default ArrayBuffer allocator");
    return nullptr;
  }
}

__declspec(dllexport) void* gov8_aba_new_host(uintptr_t id) {
  ClearErr();
  if (id == 0 || gov8_array_buffer_allocator_impl::dispatcher == nullptr) {
    SetErr("invalid host ArrayBuffer allocator argument");
    return nullptr;
  }
  try {
    auto allocator =
        std::make_shared<gov8_array_buffer_allocator_impl::HostAllocator>(id);
    return new gov8_array_buffer_allocator_impl::AllocatorWrap(
        std::move(allocator));
  } catch (...) {
    SetErr("C++ exception creating host ArrayBuffer allocator");
    return nullptr;
  }
}

// Creates a standalone backing store whose native wrapper retains the
// isolate's allocator shared reference. This is ownership-equivalent to a
// separate gov8_aba_clone wrapper, but lets backing-store disposal release the
// store first and allocator second in one native call.
__declspec(dllexport) void* gov8_aba_backing_store_new(
    v8::Isolate* iso, int64_t byte_length, void* allocator_wire) {
  ClearErr();
  auto* allocator = gov8_array_buffer_allocator_impl::Checked(allocator_wire);
  if (iso == nullptr || byte_length < 0 || allocator == nullptr) {
    SetErr("invalid allocator-aware backing store argument");
    return nullptr;
  }
  void* wire = gov8_backing_store_new(iso, byte_length);
  if (wire == nullptr) return nullptr;
  auto* store = gov8_buf_impl::AsBs(wire);
  if (store == nullptr) {
    gov8_backing_store_dispose(wire);
    SetErr("invalid allocator-aware backing store result");
    return nullptr;
  }
  try {
    store->retained_owner =
        new std::shared_ptr<v8::ArrayBuffer::Allocator>(allocator->allocator);
    store->release_retained_owner =
        gov8_array_buffer_allocator_impl::ReleaseRetainedAllocator;
    return wire;
  } catch (...) {
    gov8_backing_store_dispose(wire);
    SetErr("C++ exception retaining backing store allocator");
    return nullptr;
  }
}

__declspec(dllexport) void* gov8_aba_clone(void* wire) {
  ClearErr();
  auto* wrap = gov8_array_buffer_allocator_impl::Checked(wire);
  if (wrap == nullptr) {
    SetErr("invalid ArrayBuffer allocator");
    return nullptr;
  }
  try {
    return new gov8_array_buffer_allocator_impl::AllocatorWrap(wrap->allocator);
  } catch (...) {
    SetErr("C++ exception cloning ArrayBuffer allocator");
    return nullptr;
  }
}

__declspec(dllexport) int64_t gov8_aba_use_count(void* wire) {
  ClearErr();
  auto* wrap = gov8_array_buffer_allocator_impl::Checked(wire);
  if (wrap == nullptr) {
    SetErr("invalid ArrayBuffer allocator");
    return kErrBadArg;
  }
  return static_cast<int64_t>(wrap->allocator.use_count());
}

__declspec(dllexport) int64_t gov8_aba_dispose(void* wire) {
  ClearErr();
  auto* wrap = gov8_array_buffer_allocator_impl::Checked(wire);
  if (wrap == nullptr) {
    SetErr("invalid ArrayBuffer allocator");
    return kErrBadArg;
  }
  wrap->magic = 0;
  delete wrap;
  return kOk;
}

}  // extern "C"
