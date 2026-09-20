//! CSSOM table-internal boxes derived from the table's actual layout tracks.
use crate::{BaseDocument, NodeId};
use crate::document::BoundingRect;
use crate::node::SpecialElementData;
use style::values::specified::box_::DisplayInside;

impl BaseDocument {
    /// Table rows and row groups do not own Taffy boxes. Their authoritative
    /// geometry is the track interval assigned by the table layout algorithm,
    /// not the union of cell boxes (which is wrong in the presence of rowspan).
    pub fn table_internal_rect(&self, node_id: NodeId) -> Option<BoundingRect> {
        let node = self.get_node(node_id)?;
        let display = node.primary_styles()?.clone_display();
        if !matches!(display.inside(), DisplayInside::TableRow | DisplayInside::TableRowGroup | DisplayInside::TableHeaderGroup | DisplayInside::TableFooterGroup) { return None; }
        let mut ancestor = node.parent;
        while let Some(id) = ancestor {
            let table = self.get_node(id)?;
            if let Some(element) = table.element_data() {
                if let SpecialElementData::TableRoot(ctx) = &element.special_data {
                    let info = ctx.computed_grid_info.borrow();
                    let info = info.as_ref()?;
                    let mut first = None;
                    let mut last = None;
                    for (index, row) in ctx.rows.iter().enumerate() {
                        let mut current = Some(row.node_id);
                        while let Some(row_ancestor) = current {
                            if row_ancestor == node_id {
                                first.get_or_insert(index);
                                last = Some(index);
                                break;
                            }
                            if row_ancestor == id { break; }
                            current = self.get_node(row_ancestor)?.parent;
                        }
                    }
                    let start = info.rows.positions.get(first?)?.start;
                    let end = info.rows.positions.get(last?)?.end;
                    let left = info.columns.positions.first()?.start;
                    let right = info.columns.positions.last()?.end;
                    let origin = table.unrounded_absolute_position(0.0, 0.0);
                    let scroll = self.viewport_scroll();
                    let snap = |value: f64| (value * 64.0).round() / 64.0;
                    return Some(BoundingRect {
                        x: snap(origin.x as f64 + left as f64 - scroll.x),
                        y: snap(origin.y as f64 + start as f64 + *ctx.caption_block_start.borrow() as f64 - scroll.y),
                        width: snap((right - left) as f64),
                        height: snap((end - start) as f64),
                    });
                }
            }
            ancestor = table.parent;
        }
        None
    }
}
