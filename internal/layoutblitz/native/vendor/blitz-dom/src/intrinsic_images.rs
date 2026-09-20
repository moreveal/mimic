use crate::layout::damage::ALL_DAMAGE;
use crate::node::{ImageData, SpecialElementData};
use crate::{BaseDocument, NodeId};

impl BaseDocument {
    /// Publish already-decoded canonical dimensions without initiating network
    /// work, allocating pixels, or rewriting author attributes.
    pub fn set_canonical_image(
        &mut self,
        id: NodeId,
        width: u32,
        height: u32,
        complete: bool,
    ) -> Result<bool, ()> {
        let node = self.get_node_mut(id).ok_or(())?;
        let element = node.element_data_mut().ok_or(())?;
        if element.name.local.as_ref() != "img" {
            return Err(());
        }
        if matches!(&element.special_data, SpecialElementData::Image(data)
            if matches!(**data, ImageData::Intrinsic {width: w, height: h, complete: c} if w == width && h == height && c == complete))
        {
            return Ok(false);
        }
        element.special_data = SpecialElementData::Image(Box::new(ImageData::Intrinsic {
            width,
            height,
            complete,
        }));
        node.clear_layout_cache();
        node.insert_damage(ALL_DAMAGE);
        node.mark_ancestors_dirty();
        Ok(true)
    }
}
