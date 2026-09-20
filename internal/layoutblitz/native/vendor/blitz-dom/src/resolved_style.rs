//! Computation of *resolved* CSS property values, as exposed to JavaScript by
//! `getComputedStyle()`.
//!
//! For most properties the resolved value is the stylo computed value. For
//! layout-dependent properties (`width`/`height`, grid track sizes) it is the
//! *used* value, computed from the most recent layout.

use cssparser::{Parser, ParserInput};
use selectors::matching::QuirksMode;
use style::computed_values::box_sizing::T as BoxSizing;
use style::computed_values::position::T as Position;
use style::parser::ParserContext;
use style::properties::declaration_block::{Importance, parse_style_attribute};
use style::properties::{
    ComputedValues, NonCustomPropertyId, PropertyDeclaration, PropertyDeclarationBlock, PropertyId,
    ShorthandId, SourcePropertyDeclaration, parse_one_declaration_into,
};
use style::servo_arc::Arc as ServoArc;
use style::stylesheets::supports_rule::parse_condition_or_declaration;
use style::stylesheets::{CssRuleType, Origin, OriginSet, UrlExtraData};
use style::stylist::RegisterCustomPropertyResult;
use style::values::computed::length::CSSPixelLength;
use style::values::computed::{GridTemplateAreas, GridTemplateComponent, LengthPercentage};
use style::values::generics::position::{Inset as GenericInset, PreferredRatio};
use style::values::resolved;
use style::values::specified::box_::{DisplayInside, DisplayOutside};
use style_traits::{CssStringWriter, ParsingMode, ToCss};

use blitz_traits::node_id::NodeId;
use url::Url;

use crate::BaseDocument;
use crate::layout::replaced::is_replaced_element;
use crate::local_name;

/// Serialize a used length (in CSS pixels) the way stylo serializes computed lengths
fn format_px(px: f32) -> String {
    CSSPixelLength::new(px).to_css_string()
}

fn has_top_level_comma(value: &str) -> bool {
    let mut depth = 0u32;
    let mut quote = None;
    let mut escaped = false;
    for ch in value.chars() {
        if escaped { escaped = false; continue; }
        if ch == '\\' { escaped = true; continue; }
        if let Some(active) = quote {
            if ch == active { quote = None; }
            continue;
        }
        match ch {
            '\'' | '"' => quote = Some(ch),
            '(' | '[' => depth += 1,
            ')' | ']' => depth = depth.saturating_sub(1),
            ',' if depth == 0 => return true,
            _ => {}
        }
    }
    false
}

/// Resolve a computed inset value to CSS pixels against the given percentage
/// basis. Returns `None` for `auto` (and unsupported anchor functions).
fn resolve_inset<P>(val: &GenericInset<P, LengthPercentage>, basis: f32) -> Option<f32> {
    match val {
        GenericInset::LengthPercentage(lp) => Some(lp.resolve(CSSPixelLength::new(basis)).px()),
        _ => None,
    }
}

/// Serialize a transform matrix component rounded to 6 decimal places (as
/// browsers do when serializing resolved transform matrices)
fn format_matrix_component(v: f64) -> String {
    let rounded = (v * 1e6).round() / 1e6;
    // Avoid "-0"
    let rounded = if rounded == 0.0 { 0.0 } else { rounded };
    format!("{rounded}")
}

/// Serialize a used transform matrix the way `getComputedStyle()` does
/// (`matrix(...)` for 2D transforms, `matrix3d(...)` otherwise).
fn format_matrix(m: &euclid::default::Transform3D<f64>, is_3d: bool) -> String {
    let c = format_matrix_component;
    if is_3d {
        format!(
            "matrix3d({}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {})",
            c(m.m11),
            c(m.m12),
            c(m.m13),
            c(m.m14),
            c(m.m21),
            c(m.m22),
            c(m.m23),
            c(m.m24),
            c(m.m31),
            c(m.m32),
            c(m.m33),
            c(m.m34),
            c(m.m41),
            c(m.m42),
            c(m.m43),
            c(m.m44)
        )
    } else {
        format!(
            "matrix({}, {}, {}, {}, {}, {})",
            c(m.m11),
            c(m.m12),
            c(m.m21),
            c(m.m22),
            c(m.m41),
            c(m.m42)
        )
    }
}

/// Check whether `name` names a CSS property supported by the style engine.
/// Custom properties (`--*`) are not considered "supported" here, matching the
/// behaviour of the `in` operator on `CSSStyleDeclaration` objects in browsers.
pub fn css_property_is_supported(name: &str) -> bool {
    matches!(
        PropertyId::parse_enabled_for_all_content(name),
        Ok(property_id) if !matches!(property_id, PropertyId::Custom(_))
    )
}

/// Parse a CSS `transform` list into a 4x4 matrix, as required by the
/// `DOMMatrix(DOMString)` constructor and `DOMMatrix.setMatrixValue()`
/// (<https://drafts.fxtf.org/geometry/#parse-a-string-into-an-abstract-matrix>).
///
/// Returns the 16 matrix components in column-major order (`m11, m12, ...,
/// m44`) and whether the list contained only 2D transform functions.
/// Returns `None` if the string fails to parse as a transform list or uses
/// relative lengths (which cannot be resolved without a context).
pub fn parse_transform_matrix(value: &str) -> Option<([f64; 16], bool)> {
    use style::properties::longhands::transform;
    // Transform lists cannot contain URLs, so any base URL will do
    let url_data = UrlExtraData(ServoArc::new(Url::parse("about:blank").unwrap()));
    let context = ParserContext::new(
        Origin::Author,
        &url_data,
        Some(CssRuleType::Style),
        ParsingMode::DEFAULT,
        QuirksMode::NoQuirks,
        Default::default(),
        None,
        None,
        Default::default(),
    );
    let mut input = ParserInput::new(value);
    let mut parser = Parser::new(&mut input);
    let transform = parser
        .parse_entirely(|t| transform::parse(&context, t))
        .ok()?;
    let (m, is_3d) = transform.to_transform_3d_matrix_f64(None).ok()?;
    Some((
        [
            m.m11, m.m12, m.m13, m.m14, m.m21, m.m22, m.m23, m.m24, m.m31, m.m32, m.m33, m.m34,
            m.m41, m.m42, m.m43, m.m44,
        ],
        !is_3d,
    ))
}

/// The names of the properties exposed by the `CSSStyleDeclaration` returned
/// from `getComputedStyle()` (its indexed properties): every enabled longhand,
/// sorted alphabetically.
pub fn resolved_style_property_names() -> &'static [&'static str] {
    static NAMES: std::sync::OnceLock<Vec<&'static str>> = std::sync::OnceLock::new();
    NAMES.get_or_init(|| {
        let mut names: Vec<&'static str> = NonCustomPropertyId::iter()
            .filter_map(|id| id.as_longhand())
            .map(|longhand| longhand.name())
            .filter(|name| PropertyId::parse_enabled_for_all_content(name).is_ok())
            .collect();
        names.sort_unstable();
        names
    })
}

impl BaseDocument {
    /// Check whether `value` is a valid value for the CSS property `property`.
    /// Used by CSSOM APIs (`element.style.setProperty` and friends), which must
    /// ignore invalid declarations.
    pub fn css_declaration_is_valid(&self, property: &str, value: &str) -> bool {
        let Ok(property_id) = PropertyId::parse_enabled_for_all_content(property) else {
            return false;
        };
        let mut declarations = SourcePropertyDeclaration::default();
        parse_one_declaration_into(
            &mut declarations,
            property_id,
            value,
            Origin::Author,
            &self.url.url_extra_data(),
            None,
            ParsingMode::DEFAULT,
            QuirksMode::NoQuirks,
            CssRuleType::Style,
        )
        .is_ok()
    }

    /// Parse a `style` attribute string into a [`PropertyDeclarationBlock`].
    /// Invalid declarations are dropped (per CSS error recovery).
    fn parse_style_attr_block(&self, style_attr: &str) -> PropertyDeclarationBlock {
        parse_style_attribute(
            style_attr,
            &self.url.url_extra_data(),
            None,
            QuirksMode::NoQuirks,
            CssRuleType::Style,
        )
    }

    /// The canonical CSSOM serialization of a `style` attribute string
    /// (`CSSStyleDeclaration.cssText` getter).
    pub fn style_attr_serialize(&self, style_attr: &str) -> String {
        let block = self.parse_style_attr_block(style_attr);
        let mut css = CssStringWriter::new();
        let _ = block.to_css(&mut css);
        css
    }

    /// The canonical serialization of `property`'s value in a `style` attribute
    /// string (`CSSStyleDeclaration.getPropertyValue()`). Handles shorthands.
    /// Returns an empty string if the property is not set or not recognised.
    pub fn style_attr_get_property(&self, style_attr: &str, property: &str) -> String {
        let Ok(property_id) = PropertyId::parse_enabled_for_all_content(property) else {
            return String::new();
        };
        let block = self.parse_style_attr_block(style_attr);
        let mut css = CssStringWriter::new();
        let _ = block.property_value_to_css(&property_id, &mut css);
        css
    }

    /// Set `property` to `value` in a `style` attribute string
    /// (`CSSStyleDeclaration.setProperty()`), expanding shorthands.
    ///
    /// Returns the new (canonically serialized) style attribute, or `None` if
    /// the declaration was invalid (in which case the style is unchanged, per
    /// CSSOM). An empty `value` removes the property.
    pub fn style_attr_set_property(
        &self,
        style_attr: &str,
        property: &str,
        value: &str,
        important: bool,
    ) -> Option<String> {
        let property_id = PropertyId::parse_enabled_for_all_content(property).ok()?;
        let mut block = self.parse_style_attr_block(style_attr);

        if value.trim().is_empty() {
            if let Some(first_declaration) = block.first_declaration_to_remove(&property_id) {
                block.remove_property(&property_id, first_declaration);
            }
        } else {
            let mut source = SourcePropertyDeclaration::default();
            parse_one_declaration_into(
                &mut source,
                property_id,
                value,
                Origin::Author,
                &self.url.url_extra_data(),
                None,
                ParsingMode::DEFAULT,
                QuirksMode::NoQuirks,
                CssRuleType::Style,
            )
            .ok()?;
            let importance = if important {
                Importance::Important
            } else {
                Importance::Normal
            };
            block.extend(source.drain(), importance);
        }

        let mut css = CssStringWriter::new();
        let _ = block.to_css(&mut css);
        Some(css)
    }

    /// Remove `property` from a `style` attribute string
    /// (`CSSStyleDeclaration.removeProperty()`), removing all longhands if it
    /// is a shorthand.
    ///
    /// Returns the new (canonically serialized) style attribute and the removed
    /// property's previous serialized value.
    pub fn style_attr_remove_property(
        &self,
        style_attr: &str,
        property: &str,
    ) -> Option<(String, String)> {
        let property_id = PropertyId::parse_enabled_for_all_content(property).ok()?;
        let mut block = self.parse_style_attr_block(style_attr);

        let mut removed_value = CssStringWriter::new();
        let _ = block.property_value_to_css(&property_id, &mut removed_value);
        if let Some(first_declaration) = block.first_declaration_to_remove(&property_id) {
            block.remove_property(&property_id, first_declaration);
        }

        let mut css = CssStringWriter::new();
        let _ = block.to_css(&mut css);
        Some((css, removed_value))
    }

    /// Evaluate a `@supports` condition (e.g. `(display: grid)`,
    /// `selector(:hover)`) or a bare declaration (e.g. `display: grid`), as
    /// used by the CSSOM `CSS.supports(conditionText)` API. Returns `false`
    /// for unparseable conditions.
    pub fn css_supports_condition(&self, condition: &str) -> bool {
        let mut input = ParserInput::new(condition);
        let mut parser = Parser::new(&mut input);
        let Ok(condition) = parser.parse_entirely(parse_condition_or_declaration) else {
            return false;
        };

        let url_data = self.url.url_extra_data();
        let context = ParserContext::new(
            Origin::Author,
            &url_data,
            Some(CssRuleType::Style),
            ParsingMode::DEFAULT,
            QuirksMode::NoQuirks,
            Default::default(),
            None,
            None,
            Default::default(),
        );
        condition.eval(&context)
    }

    /// Register a custom property via script (`CSS.registerProperty()`).
    /// On success all styles are invalidated so that declarations of the
    /// property are re-parsed against the new registration.
    pub fn register_custom_property(
        &mut self,
        name: &str,
        syntax: &str,
        inherits: bool,
        initial_value: Option<&str>,
    ) -> RegisterCustomPropertyResult {
        let url_data = self.url.url_extra_data();
        let result =
            self.stylist
                .register_custom_property(&url_data, name, syntax, inherits, initial_value);
        if matches!(result, RegisterCustomPropertyResult::SuccessfullyRegistered) {
            self.stylist
                .force_stylesheet_origins_dirty(OriginSet::all());
        }
        result
    }

    /// Compute the resolved value of a CSS property for the given node, as exposed
    /// by `getComputedStyle()`. Returns an empty string for unknown properties and
    /// for nodes without styles.
    ///
    /// Layout-dependent properties (`width`/`height`, `grid-template-rows`/`columns`)
    /// resolve to *used* values, so [`resolve`](Self::resolve) should be called
    /// before this method to ensure layout is up to date.
    pub fn resolved_style_value(&self, node_id: NodeId, property_name: &str) -> String {
        self.with_resolved_styles(node_id, |node, styles, has_stored_styles| {
            self.serialize_resolved_style(node_id, node, styles, has_stored_styles, property_name)
        }).unwrap_or_default()
    }

    /// Read a set of properties from one authoritative computed result. Hidden
    /// descendants need on-demand style resolution, but it is performed once
    /// for the entire batch and never installed as a fake layout primary style.
    pub fn resolved_style_values(&self, node_id: NodeId, property_names: &[&str]) -> Vec<String> {
        self.with_resolved_styles(node_id, |node, styles, has_stored_styles| {
            property_names.iter().map(|name| {
                self.serialize_resolved_style(node_id, node, styles, has_stored_styles, name)
            }).collect()
        }).unwrap_or_else(|| vec![String::new(); property_names.len()])
    }

    fn with_resolved_styles<R>(&self, node_id: NodeId, read: impl FnOnce(&crate::node::Node, &ComputedValues, bool) -> R) -> Option<R> {
        let node = self.get_node(node_id)?;
        // Elements inside a `display: none` subtree are skipped by the style
        // traversal, so their style has to be computed on demand.
        let undisplayed_styles;
        let stored_styles = node.primary_styles();
        let styles: &ComputedValues = match &stored_styles {
            Some(styles) => styles,
            None => match self.resolve_undisplayed_style(node_id) {
                Some(styles) => {
                    undisplayed_styles = styles;
                    &undisplayed_styles
                }
                None => return None,
            },
        };

        Some(read(node, styles, stored_styles.is_some()))
    }

    fn serialize_resolved_style(&self, node_id: NodeId, node: &crate::node::Node, styles: &ComputedValues, has_stored_styles: bool, property_name: &str) -> String {
        // Normalize aliases and logical longhands before the used-value cases.
        // ComputedValues stores physical properties; its generic string getter
        // alone does not apply CSSOM's logical used-size/edge mapping.
        let property_name = match property_name {
            "-webkit-logical-width" => "inline-size",
            "-webkit-logical-height" => "block-size",
            "-webkit-min-logical-width" => "min-inline-size",
            "-webkit-min-logical-height" => "min-block-size",
            "-webkit-max-logical-width" => "max-inline-size",
            "-webkit-max-logical-height" => "max-block-size",
            "-webkit-border-before" => "border-block-start",
            "-webkit-border-after" => "border-block-end",
            "-webkit-border-start" => "border-inline-start",
            "-webkit-border-end" => "border-inline-end",
            _ => property_name,
        };
        let parsed = PropertyId::parse_enabled_for_all_content(property_name).ok();
        let property_name = match parsed.as_ref() {
            Some(id) => match id.longhand_id() {
                Some(longhand) => longhand.to_physical(styles.writing_mode).name(),
                None => match id.as_shorthand() {
                    Ok(shorthand) => shorthand.name(),
                    Err(_) => property_name,
                },
            },
            None => property_name,
        };
        let display = styles.clone_display();
        // Non-atomic inline elements are laid out as style spans within their
        // inline root's text layout rather than as boxes of their own, so their
        // layout-dependent properties resolve to computed (not used) values.
        let is_non_atomic_inline = display.outside() == DisplayOutside::Inline
            && display.inside() == DisplayInside::Flow
            && !node.flags.is_inline_root()
            && node.element_data().is_none_or(|data| {
                let tag = &data.name.local;
                !(is_replaced_element(tag)
                    || *tag == local_name!("input")
                    || *tag == local_name!("textarea")
                    || *tag == local_name!("button"))
            });
        let generates_box =
            node.flags.is_in_document() && !display.is_none() && has_stored_styles;
        let has_layout_box = generates_box && !is_non_atomic_inline;

        // These CSSOM shorthands include their initial-valued components,
        // unlike the declaration serializer which is allowed to omit them.
        let read = |name: &str| self.serialize_resolved_style(node_id, node, styles, has_stored_styles, name);
        match property_name {
            // The embedding producer eligibility check rejects all authored
            // declarations of this Servo-unsupported property, including inline
            // and escaped identifiers. Only its inherited-free initial state
            // reaches native publication; no legacy cascade is needed per read.
            "content-visibility" => return "visible".to_string(),
            "text-decoration" if read("text-decoration-line") == "none" => return "none".to_string(),
            "grid" => return ["grid-template-rows", "grid-template-columns", "grid-template-areas", "grid-auto-flow", "grid-auto-rows", "grid-auto-columns"].map(read).join(" / "),
            "background" => {
                let images = read("background-image");
                // Each layer has independently cascaded longhands. The common
                // single-layer case needs full CSSOM output, including defaults;
                // multi-layer declarations retain Stylo's serializer below.
                if !has_top_level_comma(&images) {
                    return format!("{} {} {} {} {} / {} {} {}", read("background-color"), images, read("background-repeat"), read("background-attachment"), read("background-position"), read("background-size"), read("background-origin"), read("background-clip"));
                }
            }
            "flex-flow" => return format!("{} {}", read("flex-direction"), read("flex-wrap")),
            "list-style" => return format!("{} {} {}", read("list-style-position"), read("list-style-image"), read("list-style-type")),
            "outline" => return format!("{} {} {}", read("outline-color"), read("outline-style"), read("outline-width")),
            "border-top" | "border-right" | "border-bottom" | "border-left" => return format!("{} {} {}", read(&format!("{property_name}-width")), read(&format!("{property_name}-style")), read(&format!("{property_name}-color"))),
            "border" => {
                let top = read("border-top");
                return if ["border-right", "border-bottom", "border-left"].iter().all(|side| read(side) == top) { top } else { String::new() };
            }
            "border-block-start" | "border-block-end" | "border-inline-start" | "border-inline-end" => return format!("{} {} {}", read(&format!("{property_name}-width")), read(&format!("{property_name}-style")), read(&format!("{property_name}-color"))),
            "border-block" | "border-inline" => {
                let start = read(&format!("{property_name}-start"));
                return if start == read(&format!("{property_name}-end")) { start } else { String::new() };
            }
            "transform-origin" => {
                let origin = &styles.get_box().transform_origin;
                let size = if has_layout_box { node.final_layout().size } else { taffy::Size::ZERO };
                let x = format_px(origin.horizontal.resolve(CSSPixelLength::new(size.width)).px());
                let y = format_px(origin.vertical.resolve(CSSPixelLength::new(size.height)).px());
                let depth = origin.depth.to_css_string();
                return if depth == "0px" { format!("{x} {y}") } else { format!("{x} {y} {depth}") };
            }
            "perspective-origin" => {
                let origin = &styles.get_box().perspective_origin;
                let size = if has_layout_box { node.final_layout().size } else { taffy::Size::ZERO };
                return format!("{} {}", format_px(origin.horizontal.resolve(CSSPixelLength::new(size.width)).px()), format_px(origin.vertical.resolve(CSSPixelLength::new(size.height)).px()));
            }
            _ => {}
        }

        // Layout-dependent "used value" special cases
        match property_name {
            "grid-template-columns" | "grid-template-rows"
                if display.inside() == DisplayInside::Grid =>
            {
                if let Some(info) =
                    node.element_data()
                        .and_then(|data| match &data.detailed_layout_info {
                            taffy::DetailedLayoutInfo::Grid(info) => Some(info),
                            _ => None,
                        })
                {
                    return if property_name == "grid-template-columns" {
                        info.grid_template_columns()
                    } else {
                        info.grid_template_rows()
                    };
                }
            }
            // Browsers serialize the `grid-template` shorthand from the computed
            // track lists, except that a `none` track list on a grid container
            // is replaced by the used (implicit) track sizes.
            "grid-template" if display.inside() == DisplayInside::Grid => {
                let pos_styles = styles.get_position();
                let rows_are_none =
                    matches!(pos_styles.grid_template_rows, GridTemplateComponent::None);
                let columns_are_none = matches!(
                    pos_styles.grid_template_columns,
                    GridTemplateComponent::None
                );
                let info = node
                    .element_data()
                    .and_then(|data| match &data.detailed_layout_info {
                        taffy::DetailedLayoutInfo::Grid(info) => Some(info),
                        _ => None,
                    });
                if let Some(info) = info
                    && (rows_are_none || columns_are_none)
                    && matches!(pos_styles.grid_template_areas, GridTemplateAreas::None)
                {
                    let rows = if rows_are_none {
                        info.grid_template_rows()
                    } else {
                        pos_styles.grid_template_rows.to_css_string()
                    };
                    let columns = if columns_are_none {
                        info.grid_template_columns()
                    } else {
                        pos_styles.grid_template_columns.to_css_string()
                    };
                    return format!("{rows} / {columns}");
                }
            }
            "width" | "height" if has_layout_box => {
                // Used value: the layout size interpreted according to `box-sizing`
                // (border-box size for `border-box`, content-box size for `content-box`)
                let layout = node.final_layout();
                let border_box = styles.get_position().box_sizing == BoxSizing::BorderBox;
                let size = if property_name == "width" {
                    if border_box {
                        layout.size.width
                    } else {
                        layout.size.width
                            - layout.border.left
                            - layout.border.right
                            - layout.padding.left
                            - layout.padding.right
                    }
                } else if border_box {
                    layout.size.height
                } else {
                    layout.size.height
                        - layout.border.top
                        - layout.border.bottom
                        - layout.padding.top
                        - layout.padding.bottom
                };
                return format_px(size.max(0.0));
            }
            "margin-top" | "margin-right" | "margin-bottom" | "margin-left" if has_layout_box => {
                // Used value: the margin resolved by layout (percentages and
                // `auto` margins resolved to lengths)
                let layout = node.final_layout();
                let margin = match property_name {
                    "margin-top" => layout.margin.top,
                    "margin-right" => layout.margin.right,
                    "margin-bottom" => layout.margin.bottom,
                    "margin-left" => layout.margin.left,
                    _ => unreachable!(),
                };
                return format_px(margin);
            }
            "top" | "right" | "bottom" | "left" if has_layout_box => {
                let position = styles.clone_position();
                let parent_layout = node
                    .layout_parent
                    .get()
                    .and_then(|id| self.get_node(id))
                    .map(|parent| *parent.final_layout());

                match position {
                    // Used value: the relative offset. An `auto` side resolves
                    // to the negation of the opposite side. When both sides
                    // are set the axis is overconstrained and each side
                    // resolves to its computed value instead.
                    Position::Relative => {
                        let (cb_width, cb_height) = parent_layout
                            .map(|pl| {
                                (
                                    pl.size.width
                                        - pl.border.left
                                        - pl.border.right
                                        - pl.padding.left
                                        - pl.padding.right,
                                    pl.size.height
                                        - pl.border.top
                                        - pl.border.bottom
                                        - pl.padding.top
                                        - pl.padding.bottom,
                                )
                            })
                            .unwrap_or((0.0, 0.0));
                        let pos_styles = styles.get_position();
                        let is_vertical = matches!(property_name, "top" | "bottom");
                        let basis = if is_vertical { cb_height } else { cb_width };
                        let (start, end) = if is_vertical {
                            (
                                resolve_inset(&pos_styles.top, basis),
                                resolve_inset(&pos_styles.bottom, basis),
                            )
                        } else {
                            (
                                resolve_inset(&pos_styles.left, basis),
                                resolve_inset(&pos_styles.right, basis),
                            )
                        };
                        let is_start = matches!(property_name, "top" | "left");
                        let used = match (start, end) {
                            (Some(start), Some(end)) => {
                                if is_start {
                                    start
                                } else {
                                    end
                                }
                            }
                            (Some(start), None) => {
                                if is_start {
                                    start
                                } else {
                                    -start
                                }
                            }
                            (None, Some(end)) => {
                                if is_start {
                                    -end
                                } else {
                                    end
                                }
                            }
                            (None, None) => 0.0,
                        };
                        return format_px(used);
                    }
                    // A specified (non-`auto`) inset resolves as-is (with
                    // percentages resolved against the containing block), even
                    // when overconstrained. An `auto` inset resolves to the
                    // used distance between the box's margin edge and the
                    // corresponding edge of the containing block's padding box.
                    Position::Absolute | Position::Fixed => {
                        // OOF layout publishes its containing-block owner; the
                        // canonical/layout parent is not necessarily that box.
                        let containing_layout = if node.layout_data().viewport_positioned {
                            let viewport = self.stylist.device().au_viewport_size();
                            Some(taffy::Layout {
                                size: taffy::Size { width: viewport.width.to_f32_px(), height: viewport.height.to_f32_px() },
                                ..taffy::Layout::new()
                            })
                        } else {
                            node.layout_data().geometry_parent
                                .or(node.layout_parent.get())
                                .and_then(|id| self.get_node(id))
                                .map(|parent| *parent.final_layout())
                        };
                        if let Some(pl) = containing_layout {
                            let cb_width = pl.size.width - pl.border.left - pl.border.right;
                            let cb_height = pl.size.height - pl.border.top - pl.border.bottom;

                            let pos_styles = styles.get_position();
                            let (inset, basis) = match property_name {
                                "top" => (&pos_styles.top, cb_height),
                                "bottom" => (&pos_styles.bottom, cb_height),
                                "left" => (&pos_styles.left, cb_width),
                                "right" => (&pos_styles.right, cb_width),
                                _ => unreachable!(),
                            };
                            if let Some(value) = resolve_inset(inset, basis) {
                                return format_px(value);
                            }

                            let layout = node.final_layout();
                            let margin_box_top =
                                layout.location.y - layout.margin.top - pl.border.top;
                            let margin_box_left =
                                layout.location.x - layout.margin.left - pl.border.left;
                            let margin_box_width =
                                layout.margin.left + layout.size.width + layout.margin.right;
                            let margin_box_height =
                                layout.margin.top + layout.size.height + layout.margin.bottom;
                            let used = match property_name {
                                "top" => margin_box_top,
                                "bottom" => cb_height - margin_box_top - margin_box_height,
                                "left" => margin_box_left,
                                "right" => cb_width - margin_box_left - margin_box_width,
                                _ => unreachable!(),
                            };
                            return format_px(used);
                        }
                    }
                    // `static` and `sticky` boxes resolve to the computed value
                    _ => {}
                }
            }
            // `min-width: auto` / `min-height: auto` resolve to `auto` only for
            // boxes with a preferred aspect ratio and for flex and grid items;
            // otherwise (including when no box is generated) they resolve to `0px`
            "min-width" | "min-height" => {
                let pos_styles = styles.get_position();
                let is_auto = if property_name == "min-width" {
                    pos_styles.min_width.is_auto()
                } else {
                    pos_styles.min_height.is_auto()
                };
                let has_aspect_ratio =
                    !matches!(pos_styles.aspect_ratio.ratio, PreferredRatio::None);
                let preserves_auto =
                    generates_box && (has_aspect_ratio || self.is_flex_or_grid_item(node_id));
                if is_auto && !preserves_auto {
                    return format_px(0.0);
                }
            }
            "transform" if has_layout_box => {
                let transform = &styles.get_box().transform;
                if !transform.0.is_empty() {
                    // Used value: the transform list resolved to a matrix, with
                    // percentages resolved against the border box
                    let layout = node.final_layout();
                    let reference_box = euclid::Rect::new(
                        euclid::Point2D::new(CSSPixelLength::new(0.0), CSSPixelLength::new(0.0)),
                        euclid::Size2D::new(
                            CSSPixelLength::new(layout.size.width),
                            CSSPixelLength::new(layout.size.height),
                        ),
                    );
                    if let Ok((matrix, is_3d)) =
                        transform.to_transform_3d_matrix_f64(Some(&reference_box))
                    {
                        return format_matrix(&matrix, is_3d);
                    }
                }
            }
            _ => {}
        }

        // General case: serialize the stylo computed value
        let Ok(property_id) = PropertyId::parse_enabled_for_all_content(property_name) else {
            return String::new();
        };
        match property_id.as_shorthand() {
            // Serialize shorthands from the resolved values of their longhands
            Ok(shorthand) => serialize_resolved_shorthand(styles, shorthand),
            Err(declaration_id) => styles.computed_value_to_string(declaration_id),
        }
    }

    /// Whether the node's box is a flex or grid item, i.e. its nearest ancestor
    /// that generates a box (skipping `display: contents` ancestors) is a flex
    /// or grid container.
    fn is_flex_or_grid_item(&self, node_id: NodeId) -> bool {
        let mut parent_id = self.get_node(node_id).and_then(|node| node.parent);
        while let Some(parent) = parent_id.and_then(|id| self.get_node(id)) {
            let Some(parent_styles) = parent.primary_styles() else {
                return false;
            };
            let display = parent_styles.clone_display();
            if display.is_contents() {
                parent_id = parent.parent;
                continue;
            }
            return matches!(display.inside(), DisplayInside::Flex | DisplayInside::Grid);
        }
        false
    }
}

/// Serialize the resolved value of a shorthand property by resolving each of
/// its longhands and re-serializing them as the shorthand (as
/// `getComputedStyle()` does for shorthands). Returns an empty string if the
/// longhand values cannot be represented as the shorthand.
fn serialize_resolved_shorthand(styles: &ComputedValues, shorthand: ShorthandId) -> String {
    // `all` cannot be serialized from longhands
    if shorthand == ShorthandId::All {
        return String::new();
    }

    let declarations: Vec<PropertyDeclaration> = shorthand
        .longhands()
        .map(|longhand| {
            let mut context = resolved::Context {
                style: styles,
                for_property: PropertyId::NonCustom(longhand.into()),
                current_longhand: Some(longhand),
            };
            styles.computed_or_resolved_declaration(longhand, Some(&mut context))
        })
        .collect();
    let declaration_refs: Vec<&PropertyDeclaration> = declarations.iter().collect();

    let mut css = CssStringWriter::new();
    let _ = shorthand.longhands_to_css(&declaration_refs, &mut css);
    css
}
