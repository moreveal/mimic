//! Canonical form state is distinct from the default-value attribute.
use crate::node::{SpecialElementData, TextInputData};
use crate::{BaseDocument, NodeId};
use style::selector_parser::RestyleDamage;

impl BaseDocument {
    pub fn set_canonical_control_value(&mut self, id: NodeId, value: &str, multiline: bool) {
        let node = &mut self.nodes[id];
        let has_styles = node.primary_styles().is_some();
        let element = node.element_data_mut().unwrap();
        if !matches!(&element.special_data, SpecialElementData::TextInput(data) if data.is_multiline == multiline)
        {
            element.special_data = SpecialElementData::TextInput(TextInputData::new(multiline));
        }
        let data = element.text_input_data_mut().unwrap();
        if data.editor.text() == value {
            return;
        }
        // Construction supplies current computed text styles before shaping.
        // No attribute write: [value] continues observing the default value.
        data.editor.set_text(value);
        if has_styles {
            data.editor
                .refresh_layout(&mut self.font_ctx.lock().unwrap(), &mut self.layout_ctx);
        }
        node.insert_damage(RestyleDamage::RELAYOUT);
        node.clear_layout_cache();
        let mut parent = node.parent;
        while let Some(id) = parent {
            let node = &mut self.nodes[id];
            node.clear_layout_cache();
            parent = node.parent;
        }
    }
}
