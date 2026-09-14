// HTMLDDA is a V8 object flag: a Proxy or a JS function cannot reproduce its
// typeof, truthiness and abstract equality. Allocate the native object lazily
// so the bootstrap snapshot contains only realm-local prototypes and methods.
{
  const collections = new WeakMap();
  const namedTags = new Set([
    'A',
    'BUTTON',
    'EMBED',
    'FORM',
    'IFRAME',
    'IMG',
    'INPUT',
    'MAP',
    'META',
    'OBJECT',
    'SELECT',
    'TEXTAREA',
  ]);
  const indexed = (key) =>
    typeof key === 'string' && /^(0|[1-9]\d*)$/.test(key) && Number(key) < 4294967295;
  const string = (value) => {
    if (typeof value === 'symbol') throw new TypeError('Cannot convert a Symbol value to a string');
    return String(value);
  };
  const rows = (s) => host.queryAllWithin(s.root, '*');
  const names = (id) => {
    const data = host.nodeData(id),
      attrs = data.attributes || {},
      result = [];
    if (attrs.id) result.push(attrs.id);
    if (
      data.namespaceURI === 'http://www.w3.org/1999/xhtml' &&
      namedTags.has(data.tagName) &&
      attrs.name
    )
      result.push(attrs.name);
    return result;
  };
  const matches = (s, name) =>
    name === '' ? [] : rows(s).filter((id) => names(id).includes(name));
  const named = (s, name) => {
    const ids = matches(s, name);
    if (!ids.length) return null;
    if (ids.length === 1) return wrap(ids[0]);
    let collection = s.named.get(name);
    if (!collection) {
      collection = htmlCollection(() => matches(s, name));
      s.named.set(name, collection);
    }
    return collection;
  };
  // Chrome tests an index using one DOMString conversion, then performs a
  // separate named-lookup conversion of the original argument on the other
  // branch. Keep both conversions (and their side effects) in the caller realm.
  const itemArguments = (args) => {
    if (!args.length) return [];
    const key = string(args[0]);
    return indexed(key) ? [key, true] : [string(args[0]), false];
  };
  const lookup = (s, args) => {
    if (!args.length) return null;
    return args[1] ? wrap(rows(s)[Number(args[0])]) : named(s, args[0]);
  };
  class HTMLAllCollection {
    constructor() {
      throw new TypeError('Illegal constructor');
    }
  }
  const prototype = HTMLAllCollection.prototype;
  // Order and descriptors are observable (including the inherited method
  // hiding an element named "item", while that name remains in ownKeys).
  delete prototype.constructor;
  const length = Object.getOwnPropertyDescriptor(
    {
      get length() {
        const binding = requireRealmBinding(this, 'HTMLAllCollection');
        return callRealmBinding(this, binding, 'length', []);
      },
    },
    'length',
  ).get;
  Object.defineProperty(length, 'name', { value: 'get length', configurable: true });
  markNative(length, 'length', 'get ');
  Object.defineProperty(prototype, 'length', { get: length, enumerable: true, configurable: true });
  const item = {
    item(...args) {
      const binding = requireRealmBinding(this, 'HTMLAllCollection');
      return callRealmBinding(this, binding, 'item', itemArguments(args));
    },
  }.item;
  const namedItem = {
    namedItem(name) {
      const binding = requireRealmBinding(this, 'HTMLAllCollection');
      if (!arguments.length) throw new TypeError('Not enough arguments');
      name = string(name);
      return callRealmBinding(this, binding, 'namedItem', [name]);
    },
  }.namedItem;
  for (const [key, fn] of [
    ['item', item],
    ['namedItem', namedItem],
  ]) {
    markNative(fn, key);
    Object.defineProperty(prototype, key, {
      value: fn,
      writable: true,
      enumerable: true,
      configurable: true,
    });
  }
  Object.defineProperty(prototype, 'constructor', {
    value: HTMLAllCollection,
    writable: true,
    configurable: true,
  });
  Object.defineProperty(prototype, Symbol.toStringTag, {
    value: 'HTMLAllCollection',
    configurable: true,
  });
  Object.defineProperty(prototype, Symbol.iterator, {
    value: Array.prototype.values,
    writable: true,
    configurable: true,
  });
  markNative(HTMLAllCollection, 'HTMLAllCollection');
  Object.defineProperty(globalThis, 'HTMLAllCollection', {
    value: HTMLAllCollection,
    writable: true,
    configurable: true,
  });

  const getAll = Object.getOwnPropertyDescriptor(
    {
      get all() {
        if (!(this instanceof Document)) throw new TypeError('Illegal invocation');
        let collection = collections.get(this);
        // HTMLDDA is falsy, even though it is neither null nor undefined.
        if (collection !== undefined) return collection;
        if (typeof host.createUndetectable !== 'function')
          throw new TypeError('HTMLDDA requires native engine support');
        const s = {
          root: this === document ? host.documentRootID() : elementSlot(this).nodeId,
          named: new Map(),
        };
        const descriptor = (key) => {
          if (indexed(key)) {
            const ids = rows(s),
              i = Number(key);
            return i < ids.length
              ? { value: wrap(ids[i]), writable: false, enumerable: true, configurable: true }
              : undefined;
          }
          if (typeof key !== 'string' || Reflect.has(Object.getPrototypeOf(collection), key))
            return undefined;
          const value = named(s, key);
          return value === null
            ? undefined
            : { value, writable: false, enumerable: false, configurable: true };
        };
        collection = host.createUndetectable({
          __proto__: null,
          nonMasking: true,
          call(args, construct) {
            if (construct) throw new TypeError('Illegal constructor');
            return lookup(s, itemArguments(args));
          },
          get(key) {
            const d = descriptor(key);
            return d ? [true, d.value] : [false];
          },
          getOwnPropertyDescriptor(key) {
            const d = descriptor(key);
            return d ? [true, d] : [false];
          },
          set(key) {
            return indexed(key) || descriptor(key) ? [true, true] : [false];
          },
          defineProperty(key) {
            return indexed(key) || descriptor(key) ? [true, true] : [false];
          },
          deleteProperty(key) {
            return descriptor(key) ? [true, false] : [false];
          },
          ownKeys() {
            const ids = rows(s),
              keys = ids.map((_, i) => String(i)),
              seen = new Set(keys);
            for (const id of ids)
              for (const name of names(id))
                if (!seen.has(name)) {
                  seen.add(name);
                  keys.push(name);
                }
            return keys;
          },
        });
        Object.setPrototypeOf(collection, prototype);
        registerRealmBinding(collection, 'HTMLAllCollection', {
          length: () => rows(s).length,
          item: (...args) => lookup(s, args),
          namedItem: (name) => named(s, name),
        });
        collections.set(this, collection);
        return collection;
      },
    },
    'all',
  ).get;
  Object.defineProperty(getAll, 'name', { value: 'get all', configurable: true });
  markNative(getAll, 'all', 'get ');
  Object.defineProperty(Document.prototype, 'all', {
    get: getAll,
    enumerable: true,
    configurable: true,
  });
}
