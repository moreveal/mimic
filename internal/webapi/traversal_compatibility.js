// Tree walkers traverse canonical live nodes. In particular, template content
// is a separate root and rejected subtrees must not contribute template parts.
if (typeof globalThis.TreeWalker === 'function') {
  const walkers = new WeakMap();
  const state = (walker) => {
    const value = walkers.get(walker);
    if (!value) throw new TypeError('Illegal invocation');
    return value;
  };
  function accept(s, node) {
    if (s.active) throw new DOMException('The filter is already active', 'InvalidStateError');
    if (!(s.whatToShow & (1 << (node.nodeType - 1)))) return 3;
    if (!s.filter) return 1;
    s.active = true;
    try {
      return (
        Number(typeof s.filter === 'function' ? s.filter(node) : s.filter.acceptNode(node)) & 65535
      );
    } finally {
      s.active = false;
    }
  }
  function child(s, last) {
    let node = last ? s.current.lastChild : s.current.firstChild;
    while (node) {
      const result = accept(s, node);
      if (result === 1) {
        s.current = node;
        return node;
      }
      if (result === 3) {
        const next = last ? node.lastChild : node.firstChild;
        if (next) {
          node = next;
          continue;
        }
      }
      while (node) {
        const next = last ? node.previousSibling : node.nextSibling;
        if (next) {
          node = next;
          break;
        }
        node = node.parentNode;
        if (!node || node === s.root || node === s.current) return null;
      }
    }
    return null;
  }
  function sibling(s, previous) {
    let node = s.current;
    if (node === s.root) return null;
    while (node) {
      let next = previous ? node.previousSibling : node.nextSibling;
      while (next) {
        node = next;
        const result = accept(s, node);
        if (result === 1) {
          s.current = node;
          return node;
        }
        next = result === 3 ? (previous ? node.lastChild : node.firstChild) : null;
        if (!next) next = previous ? node.previousSibling : node.nextSibling;
      }
      node = node.parentNode;
      if (!node || node === s.root || accept(s, node) === 1) return null;
    }
    return null;
  }
  const proto = globalThis.TreeWalker.prototype;
  Object.defineProperties(proto, {
    root: {
      get() {
        return state(this).root;
      },
      configurable: true,
      enumerable: true,
    },
    whatToShow: {
      get() {
        return state(this).whatToShow;
      },
      configurable: true,
      enumerable: true,
    },
    filter: {
      get() {
        return state(this).filter;
      },
      configurable: true,
      enumerable: true,
    },
    currentNode: {
      get() {
        return state(this).current;
      },
      set(node) {
        if (!isDOMNode(node)) throw new TypeError('currentNode must be a Node');
        state(this).current = node;
      },
      configurable: true,
      enumerable: true,
    },
    parentNode: {
      value: function () {
        const s = state(this);
        let node = s.current;
        while (node && node !== s.root) {
          node = node.parentNode;
          if (node && accept(s, node) === 1) {
            s.current = node;
            return node;
          }
        }
        return null;
      },
      configurable: true,
      writable: true,
    },
    firstChild: {
      value: function () {
        return child(state(this), false);
      },
      configurable: true,
      writable: true,
    },
    lastChild: {
      value: function () {
        return child(state(this), true);
      },
      configurable: true,
      writable: true,
    },
    nextSibling: {
      value: function () {
        return sibling(state(this), false);
      },
      configurable: true,
      writable: true,
    },
    previousSibling: {
      value: function () {
        return sibling(state(this), true);
      },
      configurable: true,
      writable: true,
    },
    nextNode: {
      value: function () {
        const s = state(this);
        let node = s.current,
          result = 1;
        while (node) {
          while (result !== 2 && node.firstChild) {
            node = node.firstChild;
            result = accept(s, node);
            if (result === 1) {
              s.current = node;
              return node;
            }
          }
          let next = null;
          while (node && node !== s.root) {
            if (node.nextSibling) {
              next = node.nextSibling;
              break;
            }
            node = node.parentNode;
          }
          if (!next) return null;
          node = next;
          result = accept(s, node);
          if (result === 1) {
            s.current = node;
            return node;
          }
        }
        return null;
      },
      configurable: true,
      writable: true,
    },
    previousNode: {
      value: function () {
        const s = state(this);
        let node = s.current;
        while (node && node !== s.root) {
          while (node.previousSibling) {
            node = node.previousSibling;
            let result = accept(s, node);
            while (result !== 2 && node.lastChild) {
              node = node.lastChild;
              result = accept(s, node);
            }
            if (result === 1) {
              s.current = node;
              return node;
            }
          }
          node = node.parentNode;
          if (node && accept(s, node) === 1) {
            s.current = node;
            return node;
          }
        }
        return null;
      },
      configurable: true,
      writable: true,
    },
  });
  Object.defineProperty(Document.prototype, 'createTreeWalker', {
    value: function (root, whatToShow = 0xffffffff, filter = null) {
      if (!isDOMNode(root)) throw new TypeError('root must be a Node');
      if (filter !== null && typeof filter !== 'function' && typeof filter !== 'object')
        throw new TypeError('filter must be an object');
      const walker = Object.create(proto);
      walkers.set(walker, {
        root,
        current: root,
        whatToShow: Number(whatToShow) >>> 0,
        filter,
        active: false,
      });
      return walker;
    },
    configurable: true,
    writable: true,
    enumerable: true,
  });
}
