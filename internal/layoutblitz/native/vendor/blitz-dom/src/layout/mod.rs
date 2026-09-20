//! Enable the dom to lay itself out using taffy
//!
//! In servo, style and layout happen together during traversal
//! However, in Blitz, we do a style pass then a layout pass.
//! This is slower, yes, but happens fast enough that it's not a huge issue.

use crate::node::{ComputedStyleRef, ImageData, NodeData, SpecialElementData};
use crate::{document::BaseDocument, dom_node_id, node::Node, taffy_node_id};
use markup5ever::{LocalName, local_name};
use std::cell::Ref;
use std::sync::Arc;
use style::Atom;
use style::values::computed::CSSPixelLength;
use style::values::computed::length_percentage::CalcLengthPercentage;
use stylo_taffy::TaffyStyloStyle;
use taffy::{
    BlockContext, CoreStyle as _, FlexDirection, LayoutContainingBlock, LayoutPartialTree, NodeId,
    ResolveOrZero, RoundTree, RunMode, TraversePartialTree, TraverseTree, compute_block_layout,
    compute_cached_layout, compute_flexbox_layout, compute_grid_layout, compute_leaf_layout,
    compute_oof_layout, prelude::*,
};

pub(crate) mod construct;
pub(crate) mod damage;
pub(crate) mod inline;
pub(crate) mod list;
pub(crate) mod replaced;
pub(crate) mod table;

use self::replaced::{
    IntrinsicSizes, ReplacedContext, compute_replaced_layout, is_replaced_element,
};

/// The default object size for replaced elements
/// (https://drafts.csswg.org/css-images/#default-object-size).
const DEFAULT_OBJECT_SIZE: taffy::Size<f32> = taffy::Size {
    width: 300.0,
    height: 150.0,
};

/// The intrinsic dimensions and default object size for a replaced element
/// whose intrinsic dimensions are determined by its tag: an image with no
/// loaded resource has no intrinsic dimensions and a zero default object size;
/// a canvas has an intrinsic size and aspect ratio given by its width/height
/// attributes (defaulting to 300x150); other replaced elements (video, iframe,
/// embed) have no intrinsic dimensions and the 300x150 default object size.
fn tag_intrinsic_sizes(
    tag_name: &LocalName,
    attr_size: taffy::Size<Option<f32>>,
) -> (IntrinsicSizes, taffy::Size<f32>) {
    if *tag_name == local_name!("img") || *tag_name == local_name!("svg") {
        return (IntrinsicSizes::default(), taffy::Size::ZERO);
    }
    if *tag_name == local_name!("canvas") {
        let width = attr_size.width.unwrap_or(300.0);
        let height = attr_size.height.unwrap_or(150.0);
        return (
            IntrinsicSizes {
                width: Some(width),
                height: Some(height),
                ratio: Some(width / height),
            },
            DEFAULT_OBJECT_SIZE,
        );
    }
    (IntrinsicSizes::default(), DEFAULT_OBJECT_SIZE)
}

pub(crate) fn resolve_calc_value(calc_ptr: *const (), parent_size: f32) -> f32 {
    let calc = unsafe { &*(calc_ptr as *const CalcLengthPercentage) };
    let result = calc.resolve(CSSPixelLength::new(parent_size));
    result.px()
}

impl BaseDocument {
    fn menu_list_intrinsic_size(
        &mut self,
        id: blitz_traits::node_id::NodeId,
    ) -> Option<taffy::Size<f32>> {
        let node = &self.nodes[id];
        let element = node.element_data()?;
        if element.name.local != local_name!("select")
            || element.has_attr(local_name!("multiple"))
            || element
                .attr(local_name!("size"))
                .and_then(|s| s.parse::<u32>().ok())
                .is_some_and(|n| n > 1)
        {
            return None;
        }
        let style = node.primary_styles()?;
        let scale = self.viewport.scale();
        let font_size = style.clone_font_size().used_size().px();
        let mut fonts = self.font_ctx.lock().unwrap();
        let line_height = crate::font_metrics::normal_line_height(
            &mut fonts,
            style.get_font(),
            font_size,
            scale,
        )?;
        let native_theme =
            style.clone_appearance() != style::values::specified::box_::Appearance::None;
        let mut pending = node.children.to_vec();
        let mut maximum = 0.0_f32;
        while let Some(child_id) = pending.pop() {
            let child = &self.nodes[child_id];
            let Some(data) = child.element_data() else {
                continue;
            };
            if data.name.local == local_name!("option") {
                let text = data
                    .attr(local_name!("label"))
                    .filter(|text| !text.is_empty())
                    .map(str::to_owned)
                    .unwrap_or_else(|| child.text_content());
                let text = text.split_ascii_whitespace().collect::<Vec<_>>().join(" ");
                let option_style = child
                    .primary_styles()
                    .map(|value| value.clone())
                    .unwrap_or_else(|| style.clone());
                let parley_style = crate::stylo_to_parley::style(child_id, &option_style);
                let mut builder =
                    self.layout_ctx
                        .tree_builder(&mut fonts, scale, true, &parley_style);
                builder.push_text(&text);
                let mut layout = parley::Layout::<crate::node::TextBrush>::new();
                builder.build_into(&mut layout);
                layout.break_all_lines(None);
                maximum = maximum.max(layout.width() / scale);
            } else if data.name.local == local_name!("optgroup") {
                pending.extend(child.children.iter().copied());
            }
        }
        // Windows Chrome's menu-list theme reserves start=4, end=1+15
        // (scrollbar arrow), top/bottom=1. Author border/padding are separate.
        Some(taffy::Size {
            width: maximum.ceil() + if native_theme { 20.0 } else { 0.0 },
            height: line_height + if native_theme { 2.0 } else { 0.0 },
        })
    }

    fn node_from_id(&self, node_id: taffy::prelude::NodeId) -> &Node {
        &self.nodes[dom_node_id(node_id)]
    }
    fn node_from_id_mut(&mut self, node_id: taffy::prelude::NodeId) -> &mut Node {
        &mut self.nodes[dom_node_id(node_id)]
    }

    /// The actual positioned children claimed by this containing block.
    fn out_of_flow_child_ids(&self, node_id: NodeId) -> impl Iterator<Item = NodeId> + '_ {
        self.node_from_id(node_id).layout_data().hoisted_children.iter().copied().map(taffy_node_id)
    }
}

impl BaseDocument {
    /// Run the node's layout algorithm, then lay out the out-of-flow (absolute/fixed)
    /// boxes for which it is the containing block. Must be called inside the layout
    /// cache wrapper so that cache hits do not re-run the out-of-flow pass.
    fn compute_child_layout_internal(
        &mut self,
        node_id: NodeId,
        mut inputs: taffy::tree::LayoutInput,
        block_ctx: Option<&mut BlockContext<'_>>,
    ) -> taffy::tree::LayoutOutput {
        if self.node_from_id(node_id).layout_data().skipped_details_content {
            inputs.known_dimensions.height = Some(0.0);
        }
        let mut output = self.dispatch_child_layout(node_id, inputs, block_ctx);
        if inputs.run_mode == RunMode::PerformLayout {
            compute_oof_layout(self, node_id, &mut output);
            if dom_node_id(node_id) == self.root_element().id && !output.oof_candidates.is_empty() {
                let viewport = self.stylist.device().au_viewport_size();
                let direction = self.node_from_id(node_id).layout_style().direction();
                let positioned = taffy::compute_oof_layout_for_area(self, node_id, output.oof_candidates.take(),
                    taffy::OofPositioningArea { size: taffy::Size { width: viewport.width.to_f32_px(), height: viewport.height.to_f32_px() }, offset: taffy::Point::ZERO },
                    direction, taffy::ContainingBlockClaims::ALL);
                self.add_hoisted_children(node_id, &positioned.hoisted);
                for child in positioned.hoisted.iter().copied().map(dom_node_id) {
                    self.nodes[child].layout_data_mut().viewport_positioned = true;
                }
                output.scrollable_overflow_rect = output.scrollable_overflow_rect.union(positioned.scrollable_overflow_rect);
            }
        }
        output
    }

    fn dispatch_child_layout(
        &mut self,
        node_id: NodeId,
        inputs: taffy::tree::LayoutInput,
        block_ctx: Option<&mut BlockContext<'_>>,
    ) -> taffy::tree::LayoutOutput {
        if let Some(size) = self.menu_list_intrinsic_size(dom_node_id(node_id)) {
            return compute_leaf_layout(
                inputs,
                &self.nodes[dom_node_id(node_id)].layout_style(),
                resolve_calc_value,
                |_, _| size,
            );
        }
        let node = &mut self.nodes[dom_node_id(node_id)];
        if node.element_data().is_some_and(|element| element.name.local == local_name!("progress")) {
            let em = node.primary_styles().map_or(16.0, |style| style.clone_font_size().used_size().px());
            return compute_leaf_layout(inputs, &node.layout_style(), resolve_calc_value,
                |_, _| Size { width: 10.0 * em, height: em });
        }

        let is_text_control = node.element_data().is_some_and(|element| {
            element.name.local == local_name!("input")
                || element.name.local == local_name!("textarea")
        });
        let font_styles = node
            .primary_styles()
            .filter(|_| is_text_control)
            .map(|style| {
                use style::values::computed::font::LineHeight;

                let font_size = style.clone_font_size().used_size().px();
                let line_height = match style.clone_line_height() {
                    LineHeight::Normal => crate::font_metrics::normal_line_height(
                        &mut self.font_ctx.lock().unwrap(),
                        style.get_font(),
                        font_size,
                        self.viewport.scale(),
                    )
                    .unwrap_or(font_size * 1.2),
                    LineHeight::Number(num) => font_size * num.0,
                    LineHeight::Length(value) => value.0.px(),
                };

                let widths = crate::font_metrics::control_character_widths(
                    &mut self.font_ctx.lock().unwrap(),
                    style.get_font(),
                    font_size,
                );
                (font_size, line_height, widths)
            });
        let resolved_line_height = font_styles.map(|s| s.1);
        let control_widths = font_styles.and_then(|s| s.2);
        let control_scrollbar_reserve = node.primary_styles().filter(|_| is_text_control).map_or(0.0, |style| {
            use style::values::computed::Overflow;
            use style::properties::generated::longhands::scrollbar_width::computed_value::T as ScrollbarWidth;
            if matches!(style.clone_overflow_y(), Overflow::Auto | Overflow::Scroll) {
                match style.clone_scrollbar_width() {
                    ScrollbarWidth::None => 0.0,
                    ScrollbarWidth::Thin => 10.0,
                    ScrollbarWidth::Auto => 15.0,
                }
            } else { 0.0 }
        });

        match &mut node.data {
            NodeData::Text(data) => {
                // With the new "inline context" architecture all text nodes should be wrapped in an "inline layout context"
                // and should therefore never be measured individually.
                #[cfg(feature = "tracing")]
                tracing::error!(
                    node_id = ?dom_node_id(node_id),
                    data = ?data,
                    "Tried to lay out text node individually",
                );

                #[cfg(not(feature = "tracing"))]
                let _ = data;

                taffy::LayoutOutput::HIDDEN
                // unreachable!();

                // compute_leaf_layout(inputs, &node.style, |known_dimensions, available_space| {
                //     let context = TextContext {
                //         text_content: &data.content.trim(),
                //         writing_mode: WritingMode::Horizontal,
                //     };
                //     let font_metrics = FontMetrics {
                //         char_width: 8.0,
                //         char_height: 16.0,
                //     };
                //     text_measure_function(
                //         known_dimensions,
                //         available_space,
                //         &context,
                //         &font_metrics,
                //     )
                // })
            }
            NodeData::Element(element_data) | NodeData::AnonymousBlock(element_data) => {
                // TODO: deduplicate with single-line text input
                if *element_data.name.local == *"textarea" {
                    // Chrome's Windows classic-control theme reserves its
                    // scrollbar width in the intrinsic cols size even before
                    // content overflows. This is not border or content padding.
                    let rows = element_data
                        .attr(local_name!("rows"))
                        .and_then(|val| val.parse::<u32>().ok())
                        .filter(|&n| n > 0)
                        .map(|n| n as f32)
                        .unwrap_or(2.0);

                    let cols = element_data
                        .attr(local_name!("cols"))
                        .and_then(|val| val.parse::<u32>().ok())
                        .filter(|&n| n > 0)
                        .map(|n| n as f32)
                        .unwrap_or(20.0);

                    return compute_leaf_layout(
                        inputs,
                        &node.layout_style(),
                        resolve_calc_value,
                        |_known_size, _available_space| taffy::Size {
                            width: (cols * control_widths.map_or(0.0, |widths| widths.0)).ceil()
                                + control_scrollbar_reserve,
                            height: resolved_line_height.unwrap_or(16.0) * rows,
                        },
                    );
                }

                if *element_data.name.local == *"input" {
                    match element_data.attr(local_name!("type")) {
                        // if the input type is hidden, hide it
                        Some("hidden") => {
                            return taffy::LayoutOutput::HIDDEN;
                        }
                        Some("checkbox") => {
                            return compute_leaf_layout(
                                inputs,
                                &node.layout_style(),
                                resolve_calc_value,
                                |_known_size, _available_space| {
                                    let size = node.layout_style().size();
                                    let width = size.width.resolve_or_zero(
                                        inputs.parent_size.width,
                                        resolve_calc_value,
                                    );
                                    let height = size.height.resolve_or_zero(
                                        inputs.parent_size.height,
                                        resolve_calc_value,
                                    );
                                    let min_size = width.min(height);
                                    taffy::Size {
                                        width: min_size,
                                        height: min_size,
                                    }
                                },
                            );
                        }
                        None | Some("text" | "password" | "email" | "tel" | "url" | "search") => {
                            let columns = element_data
                                .attr(local_name!("size"))
                                .and_then(|value| value.parse::<u32>().ok())
                                .filter(|&value| value > 0)
                                .unwrap_or(20) as f32;
                            let (average, maximum) = control_widths.unwrap_or((0.0, 0.0));
                            let intrinsic_width =
                                (columns * average + (maximum - average).max(0.0)).ceil();
                            return compute_leaf_layout(
                                inputs,
                                &node.layout_style(),
                                resolve_calc_value,
                                |_known_size, _available_space| taffy::Size {
                                    width: intrinsic_width,
                                    height: resolved_line_height.unwrap_or(16.0),
                                },
                            );
                        }
                        _ => {}
                    }
                }

                if is_replaced_element(&element_data.name.local) {
                    // Width/height attributes are presentational hints mapped to the CSS
                    // width/height properties by
                    // `synthesize_presentational_hints_for_legacy_attributes`, so they
                    // are already part of the style. They are only read here for the
                    // elements whose attributes determine their *intrinsic* size
                    // (canvas, and custom widgets on canvas tags).
                    let attr_size = taffy::Size {
                        width: element_data
                            .attr(local_name!("width"))
                            .and_then(|val| val.parse::<f32>().ok()),
                        height: element_data
                            .attr(local_name!("height"))
                            .and_then(|val| val.parse::<f32>().ok()),
                    };

                    // Get the element's intrinsic dimensions and default object size
                    let (intrinsic_sizes, default_object_size) = match &element_data.special_data {
                        SpecialElementData::Image(image_data) => match &**image_data {
                            ImageData::Intrinsic { width, height, .. } => {
                                let (width, height) = (*width as f32, *height as f32);
                                if width > 0.0 && height > 0.0 {
                                    (
                                        IntrinsicSizes {
                                            width: Some(width),
                                            height: Some(height),
                                            ratio: Some(width / height),
                                        },
                                        DEFAULT_OBJECT_SIZE,
                                    )
                                } else {
                                    (IntrinsicSizes::default(), taffy::Size::ZERO)
                                }
                            }
                            ImageData::Raster(image) => {
                                let (width, height) = (image.width as f32, image.height as f32);
                                (
                                    IntrinsicSizes {
                                        width: Some(width),
                                        height: Some(height),
                                        ratio: Some(width / height),
                                    },
                                    DEFAULT_OBJECT_SIZE,
                                )
                            }
                            #[cfg(feature = "svg")]
                            ImageData::Svg(svg) => {
                                let mut width = svg.intrinsic_width();
                                let mut height = svg.intrinsic_height();
                                // An SVG with no declared dimensions and no viewBox has no
                                // intrinsic dimensions per CSS, but usvg still resolves a
                                // concrete size; use it in place of the default object size.
                                if width.is_none()
                                    && height.is_none()
                                    && svg.viewbox_aspect_ratio().is_none()
                                {
                                    let size = svg.tree.size();
                                    width = Some(size.width());
                                    height = Some(size.height());
                                }
                                (
                                    IntrinsicSizes {
                                        width,
                                        height,
                                        ratio: Some(svg.aspect_ratio()),
                                    },
                                    DEFAULT_OBJECT_SIZE,
                                )
                            }
                            ImageData::None => (IntrinsicSizes::default(), taffy::Size::ZERO),
                        },
                        SpecialElementData::Canvas(_)
                        | SpecialElementData::SubDocument(_)
                        | SpecialElementData::None => {
                            tag_intrinsic_sizes(&element_data.name.local, attr_size)
                        }
                        #[cfg(feature = "custom-widget")]
                        SpecialElementData::CustomWidget(widget_data) => {
                            let (fallback, default_object_size) =
                                tag_intrinsic_sizes(&element_data.name.local, attr_size);
                            // A canvas's content attributes determine its intrinsic size,
                            // overriding the widget-reported one; the widget-reported size
                            // in turn overrides the tag's fallback.
                            let attr_intrinsic =
                                if *element_data.name.local == local_name!("canvas") {
                                    attr_size
                                } else {
                                    taffy::Size::NONE
                                };
                            let attr_ratio = match (attr_intrinsic.width, attr_intrinsic.height) {
                                (Some(w), Some(h)) => Some(w / h),
                                _ => None,
                            };
                            let widget_sizes = widget_data.widget.intrinsic_sizes();
                            (
                                IntrinsicSizes {
                                    width: attr_intrinsic
                                        .width
                                        .or(widget_sizes.width)
                                        .or(fallback.width),
                                    height: attr_intrinsic
                                        .height
                                        .or(widget_sizes.height)
                                        .or(fallback.height),
                                    ratio: attr_ratio.or(widget_sizes.ratio).or(fallback.ratio),
                                },
                                default_object_size,
                            )
                        }
                        _ => unreachable!(),
                    };

                    let replaced_context = ReplacedContext {
                        intrinsic_sizes,
                        default_object_size,
                    };

                    return compute_replaced_layout(
                        inputs,
                        &node.layout_style(),
                        resolve_calc_value,
                        &replaced_context,
                    );
                }

                if node.flags.is_table_root() {
                    let SpecialElementData::TableRoot(context) = &self.nodes[dom_node_id(node_id)]
                        .data
                        .downcast_element()
                        .unwrap()
                        .special_data
                    else {
                        panic!("Node marked as table root but doesn't have TableContext");
                    };
                    let context = Arc::clone(context);

                    let mut output = self.compute_table_layout(dom_node_id(node_id), context, inputs);

                    // HACK: Cap scrollable overflow at node size to prevent scrolling
                    output.scrollable_overflow_rect.left = 0.0;
                    output.scrollable_overflow_rect.top = 0.0;
                    output.scrollable_overflow_rect.right =
                        output.scrollable_overflow_rect.right.min(output.size.width);
                    output.scrollable_overflow_rect.bottom = output
                        .scrollable_overflow_rect
                        .bottom
                        .min(output.size.height);

                    return output;
                }

                if node.flags.is_inline_root() {
                    return self.compute_inline_layout(dom_node_id(node_id), inputs, block_ctx);
                }

                // The default CSS file will set
                match node.taffy_display() {
                    Display::Block => compute_block_layout(self, node_id, inputs, block_ctx),
                    Display::FlowRoot => compute_block_layout(self, node_id, inputs, None),
                    Display::Flex => compute_flexbox_layout(self, node_id, inputs),
                    Display::Grid => compute_grid_layout(self, node_id, inputs),
                    Display::None => taffy::LayoutOutput::HIDDEN,
                }
            }
            NodeData::Document(_) => compute_block_layout(self, node_id, inputs, None),

            _ => taffy::LayoutOutput::HIDDEN,
        }
    }
}

impl TraversePartialTree for BaseDocument {
    type ChildIter<'a> = RefCellChildIter<'a>;

    fn child_ids(&self, node_id: NodeId) -> Self::ChildIter<'_> {
        let layout_children = self.node_from_id(node_id).layout_children.borrow(); //.unwrap().as_ref();
        RefCellChildIter::new(Ref::map(layout_children, |children| {
            children.as_ref().map(|c| c.as_slice()).unwrap_or(&[])
        }))
    }

    fn child_count(&self, node_id: NodeId) -> usize {
        self.node_from_id(node_id)
            .layout_children
            .borrow()
            .as_ref()
            .map(|c| c.len())
            .unwrap_or(0)
    }

    fn get_child_id(&self, node_id: NodeId, index: usize) -> NodeId {
        taffy_node_id(
            self.node_from_id(node_id)
                .layout_children
                .borrow()
                .as_ref()
                .unwrap()[index],
        )
    }
}
impl TraverseTree for BaseDocument {}

impl LayoutPartialTree for BaseDocument {
    type CoreContainerStyle<'a>
        = TaffyStyloStyle<ComputedStyleRef<'a>>
    where
        Self: 'a;

    type CustomIdent = Atom;

    fn get_core_container_style(&self, node_id: NodeId) -> Self::CoreContainerStyle<'_> {
        self.node_from_id(node_id).layout_style()
    }

    fn set_unrounded_layout(&mut self, node_id: NodeId, layout: &Layout) {
        *self.node_from_id_mut(node_id).unrounded_layout_mut() = *layout;
    }

    fn resolve_calc_value(&self, calc_ptr: *const (), parent_size: f32) -> f32 {
        resolve_calc_value(calc_ptr, parent_size)
    }

    #[inline(always)]
    fn compute_child_layout(
        &mut self,
        node_id: NodeId,
        inputs: taffy::LayoutInput,
    ) -> taffy::LayoutOutput {
        compute_cached_layout(self, node_id, inputs, |tree, node_id, inputs| {
            tree.compute_child_layout_internal(node_id, inputs, None)
        })
    }
}

impl LayoutContainingBlock for BaseDocument {
    type OofItemStyle<'a>
        = TaffyStyloStyle<ComputedStyleRef<'a>>
    where
        Self: 'a;

    fn get_oof_item_style(&self, node_id: NodeId) -> Self::OofItemStyle<'_> {
        self.node_from_id(node_id).layout_style()
    }

    fn clear_hoisted_children(&mut self, node_id: NodeId) {
        let owner = dom_node_id(node_id);
        let previous = std::mem::take(&mut self.nodes[owner].layout_data_mut().hoisted_children);
        for child in previous {
            if self.nodes[child].layout_data().geometry_parent == Some(owner) {
                self.nodes[child].layout_data_mut().geometry_parent = None;
                self.nodes[child].layout_data_mut().viewport_positioned = false;
            }
        }
    }
    fn add_hoisted_children(&mut self, node_id: NodeId, hoisted: &[NodeId]) {
        let owner = dom_node_id(node_id);
        for child in hoisted.iter().copied().map(dom_node_id) {
            self.nodes[child].layout_data_mut().geometry_parent = Some(owner);
            self.nodes[child].layout_data_mut().viewport_positioned = false;
            self.nodes[owner].layout_data_mut().hoisted_children.push(child);
        }
    }

    fn get_detailed_layout_info(&self, node_id: NodeId) -> &taffy::DetailedLayoutInfo<Atom> {
        self.node_from_id(node_id)
            .element_data()
            .map(|element| &element.detailed_layout_info)
            .unwrap_or(&taffy::DetailedLayoutInfo::None)
    }
}

impl taffy::CacheTree for BaseDocument {
    #[inline]
    fn cache_get(
        &mut self,
        node_id: NodeId,
        inputs: &taffy::LayoutInput,
    ) -> Option<taffy::LayoutOutput> {
        self.node_from_id_mut(node_id)
            .try_layout_data_mut()?
            .cache
            .get(inputs)
    }

    #[inline]
    fn cache_store(
        &mut self,
        node_id: NodeId,
        inputs: &taffy::LayoutInput,
        layout_output: taffy::LayoutOutput,
    ) {
        self.node_from_id_mut(node_id)
            .cache_mut()
            .store(inputs, layout_output);
    }

    #[inline]
    fn cache_clear(&mut self, node_id: NodeId) {
        self.node_from_id_mut(node_id).clear_layout_cache();
    }
}

impl taffy::LayoutBlockContainer for BaseDocument {
    type BlockContainerStyle<'a>
        = TaffyStyloStyle<ComputedStyleRef<'a>>
    where
        Self: 'a;

    type BlockItemStyle<'a>
        = TaffyStyloStyle<ComputedStyleRef<'a>>
    where
        Self: 'a;

    fn get_block_container_style(&self, node_id: NodeId) -> Self::BlockContainerStyle<'_> {
        self.get_core_container_style(node_id)
    }

    fn get_block_child_style(&self, child_node_id: NodeId) -> Self::BlockItemStyle<'_> {
        self.get_core_container_style(child_node_id)
    }

    #[inline(always)]
    fn compute_block_child_layout(
        &mut self,
        node_id: NodeId,
        inputs: taffy::LayoutInput,
        block_ctx: Option<&mut BlockContext<'_>>,
    ) -> taffy::LayoutOutput {
        compute_cached_layout(self, node_id, inputs, |tree, node_id, inputs| {
            tree.compute_child_layout_internal(node_id, inputs, block_ctx)
        })
    }
}

impl taffy::LayoutFlexboxContainer for BaseDocument {
    type FlexboxContainerStyle<'a>
        = TaffyStyloStyle<ComputedStyleRef<'a>>
    where
        Self: 'a;

    type FlexboxItemStyle<'a>
        = TaffyStyloStyle<ComputedStyleRef<'a>>
    where
        Self: 'a;

    fn get_flexbox_container_style(&self, node_id: NodeId) -> Self::FlexboxContainerStyle<'_> {
        self.get_core_container_style(node_id)
    }

    fn get_flexbox_child_style(&self, child_node_id: NodeId) -> Self::FlexboxItemStyle<'_> {
        self.get_core_container_style(child_node_id)
    }
}

impl taffy::LayoutGridContainer for BaseDocument {
    type GridContainerStyle<'a>
        = TaffyStyloStyle<ComputedStyleRef<'a>>
    where
        Self: 'a;

    type GridItemStyle<'a>
        = TaffyStyloStyle<ComputedStyleRef<'a>>
    where
        Self: 'a;

    fn get_grid_container_style(&self, node_id: NodeId) -> Self::GridContainerStyle<'_> {
        self.get_core_container_style(node_id)
    }

    fn get_grid_child_style(&self, child_node_id: NodeId) -> Self::GridItemStyle<'_> {
        self.get_core_container_style(child_node_id)
    }

    fn set_detailed_grid_info(
        &mut self,
        node_id: NodeId,
        detailed_grid_info: taffy::DetailedGridInfo<Atom>,
    ) {
        let node = self.node_from_id_mut(node_id);
        if let Some(element) = node.element_data_mut() {
            element.detailed_layout_info =
                taffy::DetailedLayoutInfo::Grid(Box::new(detailed_grid_info));
        }
    }
}

impl RoundTree for BaseDocument {
    fn get_unrounded_layout(&self, node_id: NodeId) -> Layout {
        *self.node_from_id(node_id).unrounded_layout()
    }

    fn set_final_layout(&mut self, node_id: NodeId, layout: &Layout) {
        *self.node_from_id_mut(node_id).final_layout_mut() = *layout;
    }

    fn is_out_of_flow(&self, node_id: NodeId) -> bool {
        self.node_from_id(node_id).is_out_of_flow()
    }

    fn hoisted_child_count(&self, node_id: NodeId) -> usize {
        self.out_of_flow_child_ids(node_id).count()
    }

    fn get_hoisted_child_id(&self, node_id: NodeId, index: usize) -> NodeId {
        self.out_of_flow_child_ids(node_id).nth(index).unwrap()
    }
}

impl PrintTree for BaseDocument {
    fn get_debug_label(&self, node_id: NodeId) -> &'static str {
        let node = &self.node_from_id(node_id);

        match node.data {
            NodeData::Document(_) => "DOCUMENT",
            // NodeData::Doctype { .. } => return "DOCTYPE",
            NodeData::Text { .. } => node.node_debug_str().leak(),
            NodeData::Comment { .. } => "COMMENT",
            NodeData::AnonymousBlock(_) => "ANONYMOUS BLOCK",
            NodeData::Element(_) => {
                let style = node.layout_style();
                let display = match node.taffy_display() {
                    Display::Flex => match taffy::FlexboxContainerStyle::flex_direction(&style) {
                        FlexDirection::Row | FlexDirection::RowReverse => "FLEX ROW",
                        FlexDirection::Column | FlexDirection::ColumnReverse => "FLEX COL",
                    },
                    Display::Grid => "GRID",
                    Display::Block => "BLOCK",
                    Display::FlowRoot => "FLOW ROOT",
                    Display::None => "NONE",
                };
                format!("{} ({})", node.node_debug_str(), display).leak()
            } // NodeData::ProcessingInstruction { .. } => return "PROCESSING INSTRUCTION",
        }
    }

    fn get_final_layout(&self, node_id: NodeId) -> Layout {
        *self.node_from_id(node_id).final_layout()
    }
}

// pub struct ChildIter<'a>(std::slice::Iter<'a, usize>);
// impl<'a> Iterator for ChildIter<'a> {
//     type Item = NodeId;
//     fn next(&mut self) -> Option<Self::Item> {
//         self.0.next().copied().map(NodeId::from)
//     }
// }

pub struct RefCellChildIter<'a> {
    items: Ref<'a, [crate::NodeId]>,
    idx: usize,
}
impl<'a> RefCellChildIter<'a> {
    fn new(items: Ref<'a, [crate::NodeId]>) -> RefCellChildIter<'a> {
        RefCellChildIter { items, idx: 0 }
    }
}

impl Iterator for RefCellChildIter<'_> {
    type Item = NodeId;
    fn next(&mut self) -> Option<Self::Item> {
        self.items.get(self.idx).map(|id| {
            self.idx += 1;
            taffy_node_id(*id)
        })
    }
}
