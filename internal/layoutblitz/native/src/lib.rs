//! Persistent derived document. Canonical IDs come from the Go DOM owner.
//! No HTML serialization or JavaScript producer participates in updates.
use blitz_dom::{BaseDocument, DocumentConfig, NodeId, StyleThreading};
use blitz_traits::shell::{ColorScheme, Viewport};
use markup5ever::{LocalName, Namespace, QualName};
use std::collections::HashMap;

mod ffi;

// Export the legacy fallback through the same archive so the executable has
// one Rust runtime during migration.
pub use mimic_taffy_layout::mimic_taffy_layout;

pub struct Owner {
    document: BaseDocument,
    nodes: HashMap<u64, NodeId>,
    dirty: bool,
    pub generation: u64,
}

#[derive(Debug, PartialEq)]
pub enum Error {
    UnknownNode(u64),
    DuplicateNode(u64),
    DirtyRead,
}

#[derive(Debug, Copy, Clone)]
#[repr(C)]
pub struct Rect {
    pub x: f32,
    pub y: f32,
    pub width: f32,
    pub height: f32,
}

impl Owner {
    pub fn new(root: u64, width: u32, height: u32) -> Self {
        let document = BaseDocument::new(DocumentConfig {
            viewport: Some(Viewport::new(width, height, 1.0, ColorScheme::Light)),
            style_threading: StyleThreading::Sequential,
            ..Default::default()
        });
        let root_node = document.root_node().id;
        Self {
            document,
            nodes: HashMap::from([(root, root_node)]),
            dirty: true,
            generation: 0,
        }
    }

    fn node(&self, id: u64) -> Result<NodeId, Error> {
        self.nodes.get(&id).copied().ok_or(Error::UnknownNode(id))
    }

    pub fn element(&mut self, id: u64, namespace: &str, name: &str) -> Result<(), Error> {
        if self.nodes.contains_key(&id) {
            return Err(Error::DuplicateNode(id));
        }
        let name = QualName::new(None, Namespace::from(namespace), LocalName::from(name));
        let node = self.document.mutate().create_element(name, Vec::new());
        self.nodes.insert(id, node);
        self.dirty = true;
        Ok(())
    }

    pub fn text(&mut self, id: u64, value: &str) -> Result<(), Error> {
        if self.nodes.contains_key(&id) {
            return Err(Error::DuplicateNode(id));
        }
        let node = self.document.mutate().create_text_node(value);
        self.nodes.insert(id, node);
        self.dirty = true;
        Ok(())
    }

    pub fn append(&mut self, parent: u64, child: u64) -> Result<(), Error> {
        let parent = self.node(parent)?;
        let child = self.node(child)?;
        self.document.mutate().append_children(parent, &[child]);
        self.dirty = true;
        Ok(())
    }

    pub fn attribute(
        &mut self,
        id: u64,
        namespace: &str,
        name: &str,
        value: &str,
    ) -> Result<(), Error> {
        let node = self.node(id)?;
        let name = QualName::new(None, Namespace::from(namespace), LocalName::from(name));
        self.document.mutate().set_attribute(node, name, value);
        self.dirty = true;
        Ok(())
    }

    pub fn set_text(&mut self, id: u64, value: &str) -> Result<(), Error> {
        let node = self.node(id)?;
        self.document.mutate().set_node_text(node, value);
        self.dirty = true;
        Ok(())
    }

    /// Resolve once per dirty transaction. Clean observations never traverse DOM.
    /// Animated documents must explicitly request a new lifecycle update.
    pub fn resolve(&mut self, time: f64) -> bool {
        if !self.dirty {
            return false;
        }
        self.document.resolve(time);
        self.dirty = false;
        self.generation += 1;
        true
    }

    pub fn rect(&self, id: u64) -> Result<Rect, Error> {
        if self.dirty {
            return Err(Error::DirtyRead);
        }
        let node = self.document.get_node(self.node(id)?).unwrap();
        let position = node.absolute_position(0.0, 0.0);
        let layout = node.final_layout();
        Ok(Rect {
            x: position.x,
            y: position.y,
            width: layout.size.width,
            height: layout.size.height,
        })
    }

    pub fn style(&self, id: u64, property: &str) -> Result<String, Error> {
        if self.dirty {
            return Err(Error::DirtyRead);
        }
        Ok(self.document.resolved_style_value(self.node(id)?, property))
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn fixture() -> Owner {
        let mut owner = Owner::new(1, 1280, 800);
        for (id, name) in [(2, "html"), (3, "body"), (4, "div")] {
            owner
                .element(id, "http://www.w3.org/1999/xhtml", name)
                .unwrap();
            owner.append(id - 1, id).unwrap();
        }
        owner
            .attribute(4, "", "style", "width:800px;height:32px")
            .unwrap();
        owner
    }

    #[test]
    fn persistent_mutation_and_clean_reads() {
        let mut owner = fixture();
        assert_eq!(owner.rect(4).unwrap_err(), Error::DirtyRead);
        assert!(owner.resolve(0.0));
        assert_eq!(owner.rect(4).unwrap().width, 800.0);
        assert!(!owner.resolve(0.0));
        assert_eq!(owner.generation, 1);
        owner
            .attribute(4, "", "style", "width:640px;height:40px")
            .unwrap();
        assert!(owner.resolve(0.0));
        assert_eq!(owner.rect(4).unwrap().width, 640.0);
        assert_eq!(owner.rect(4).unwrap().height, 40.0);
        assert_eq!(owner.generation, 2);
    }

    #[test]
    fn independent_documents() {
        let threads: Vec<_> = (0..4)
            .map(|_| {
                std::thread::spawn(|| {
                    let mut owner = fixture();
                    owner.resolve(0.0);
                    assert_eq!(owner.rect(4).unwrap().width, 800.0);
                })
            })
            .collect();
        for thread in threads {
            thread.join().unwrap();
        }
    }
}
