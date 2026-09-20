use crate::{BaseDocument, NodeData};
use blitz_traits::node_id::NodeId;

impl BaseDocument {
    /// Whether a retained CSSOM box belongs to content excluded from painting,
    /// hit testing and observer intersections by a closed details host.
    /// This is intentionally independent of computed visibility and box existence.
    pub fn is_in_skipped_content(&self, node_id: NodeId) -> bool {
        let mut current = Some(node_id);
        while let Some(id) = current {
            let node = &self.nodes[id];
            if matches!(node.data, NodeData::Element(_) | NodeData::AnonymousBlock(_) | NodeData::Document(_))
                && node.layout_data().skipped_details_content {
                return true;
            }
            current = node.layout_parent.get().or(node.parent);
        }
        false
    }
}
