use std::slice;
use taffy::prelude::*;

#[repr(C)]
#[derive(Clone, Copy)]
pub struct Length {
    kind: u32,
    value: f32,
}
#[repr(C)]
#[derive(Clone, Copy)]
pub struct Edges {
    top: Length,
    right: Length,
    bottom: Length,
    left: Length,
}
#[repr(C)]
#[derive(Clone, Copy)]
pub struct Track {
    kind: u32,
    value: f32,
}
#[repr(C)]
pub struct InputNode {
    id: u64,
    parent: i32,
    display: u32,
    position: u32,
    box_sizing: u32,
    flex_direction: u32,
    flex_wrap: u32,
    align_items: u32,
    align_self: u32,
    align_content: u32,
    justify_content: u32,
    justify_self: u32,
    justify_items: u32,
    width: Length,
    height: Length,
    min_width: Length,
    min_height: Length,
    max_width: Length,
    max_height: Length,
    flex_basis: Length,
    flex_grow: f32,
    flex_shrink: f32,
    margin: Edges,
    padding: Edges,
    border: Edges,
    inset: Edges,
    gap_x: Length,
    gap_y: Length,
    measure_width: f32,
    measure_height: f32,
    grid_column_offset: u32,
    grid_column_count: u32,
    grid_row_offset: u32,
    grid_row_count: u32,
    grid_column_start: i32,
    grid_column_end: i32,
    grid_row_start: i32,
    grid_row_end: i32,
}
#[repr(C)]
pub struct OutputBox {
    id: u64,
    x: f32,
    y: f32,
    width: f32,
    height: f32,
    content_width: f32,
    content_height: f32,
}

fn dimension(v: Length) -> Dimension {
    match v.kind {
        1 => Dimension::length(v.value),
        2 => Dimension::percent(v.value),
        3 => Dimension::min_content(),
        4 => Dimension::max_content(),
        5 => Dimension::content(),
        _ => Dimension::auto(),
    }
}
fn lpa(v: Length) -> LengthPercentageAuto {
    match v.kind {
        1 => LengthPercentageAuto::length(v.value),
        2 => LengthPercentageAuto::percent(v.value),
        _ => LengthPercentageAuto::auto(),
    }
}
fn lp(v: Length) -> LengthPercentage {
    match v.kind {
        2 => LengthPercentage::percent(v.value),
        _ => LengthPercentage::length(v.value),
    }
}
fn rect_lpa(v: Edges) -> Rect<LengthPercentageAuto> {
    Rect {
        top: lpa(v.top),
        right: lpa(v.right),
        bottom: lpa(v.bottom),
        left: lpa(v.left),
    }
}
fn rect_lp(v: Edges) -> Rect<LengthPercentage> {
    Rect {
        top: lp(v.top),
        right: lp(v.right),
        bottom: lp(v.bottom),
        left: lp(v.left),
    }
}
fn alignment(v: u32) -> Option<AlignItems> {
    match v {
        1 => Some(AlignItems::START),
        2 => Some(AlignItems::END),
        3 => Some(AlignItems::FLEX_START),
        4 => Some(AlignItems::FLEX_END),
        5 => Some(AlignItems::CENTER),
        6 => Some(AlignItems::BASELINE),
        7 => Some(AlignItems::STRETCH),
        _ => None,
    }
}
fn content_alignment(v: u32) -> Option<AlignContent> {
    match v {
        1 => Some(AlignContent::START),
        2 => Some(AlignContent::END),
        3 => Some(AlignContent::FLEX_START),
        4 => Some(AlignContent::FLEX_END),
        5 => Some(AlignContent::CENTER),
        7 => Some(AlignContent::STRETCH),
        8 => Some(AlignContent::SPACE_BETWEEN),
        9 => Some(AlignContent::SPACE_AROUND),
        10 => Some(AlignContent::SPACE_EVENLY),
        _ => None,
    }
}
fn track(v: Track) -> GridTemplateComponent<String> {
    match v.kind {
        1 => length(v.value),
        2 => percent(v.value),
        3 => fr(v.value),
        4 => min_content(),
        5 => max_content(),
        _ => auto(),
    }
}
fn placement(start: i32, end: i32) -> Line<GridPlacement> {
    Line {
        start: if start == 0 {
            GridPlacement::Auto
        } else {
            line(start as i16)
        },
        end: if end == 0 {
            GridPlacement::Auto
        } else if end < 0 {
            span((-end) as u16)
        } else {
            line(end as i16)
        },
    }
}
fn style(n: &InputNode, tracks: &[Track]) -> Style {
    let mut s = Style::default();
    s.display = match n.display {
        0 => Display::None,
        2 => Display::Flex,
        3 => Display::Grid,
        _ => Display::Block,
    };
    s.position = if n.position == 1 {
        Position::Absolute
    } else {
        Position::Relative
    };
    s.box_sizing = if n.box_sizing == 1 {
        BoxSizing::ContentBox
    } else {
        BoxSizing::BorderBox
    };
    s.size = Size {
        width: dimension(n.width),
        height: dimension(n.height),
    };
    s.min_size = Size {
        width: lpa(n.min_width),
        height: lpa(n.min_height),
    };
    s.max_size = Size {
        width: lpa(n.max_width),
        height: lpa(n.max_height),
    };
    s.flex_basis = dimension(n.flex_basis);
    s.flex_grow = n.flex_grow;
    s.flex_shrink = n.flex_shrink;
    s.flex_direction = match n.flex_direction {
        1 => FlexDirection::Column,
        2 => FlexDirection::RowReverse,
        3 => FlexDirection::ColumnReverse,
        _ => FlexDirection::Row,
    };
    s.flex_wrap = match n.flex_wrap {
        1 => FlexWrap::Wrap,
        2 => FlexWrap::WrapReverse,
        _ => FlexWrap::NoWrap,
    };
    s.align_items = alignment(n.align_items);
    s.align_self = alignment(n.align_self);
    s.align_content = content_alignment(n.align_content);
    s.justify_content = content_alignment(n.justify_content);
    s.justify_self = alignment(n.justify_self);
    s.justify_items = alignment(n.justify_items);
    s.margin = rect_lpa(n.margin);
    s.padding = rect_lp(n.padding);
    s.border = rect_lp(n.border);
    s.inset = rect_lpa(n.inset);
    s.gap = Size {
        width: lp(n.gap_x),
        height: lp(n.gap_y),
    };
    let cols = n.grid_column_offset as usize..(n.grid_column_offset + n.grid_column_count) as usize;
    let rows = n.grid_row_offset as usize..(n.grid_row_offset + n.grid_row_count) as usize;
    if cols.end <= tracks.len() {
        s.grid_template_columns = tracks[cols].iter().copied().map(track).collect()
    }
    if rows.end <= tracks.len() {
        s.grid_template_rows = tracks[rows].iter().copied().map(track).collect()
    }
    s.grid_column = placement(n.grid_column_start, n.grid_column_end);
    s.grid_row = placement(n.grid_row_start, n.grid_row_end);
    s
}

#[unsafe(no_mangle)]
pub unsafe extern "C" fn mimic_taffy_layout(
    nodes: *const InputNode,
    count: usize,
    tracks: *const Track,
    track_count: usize,
    viewport_width: f32,
    viewport_height: f32,
    boxes: *mut OutputBox,
    capacity: usize,
) -> i32 {
    if nodes.is_null()
        || boxes.is_null()
        || count == 0
        || capacity < count
        || track_count > 0 && tracks.is_null()
    {
        return -1;
    }
    let input = unsafe { slice::from_raw_parts(nodes, count) };
    let output = unsafe { slice::from_raw_parts_mut(boxes, capacity) };
    let track_input = if track_count == 0 {
        &[]
    } else {
        unsafe { slice::from_raw_parts(tracks, track_count) }
    };
    let mut tree = TaffyTree::<()>::new();
    let mut ids = Vec::with_capacity(count);
    for n in input {
        let mut s = style(n, track_input);
        if n.measure_width >= 0.0 && s.size.width.is_auto() {
            s.size.width = Dimension::length(n.measure_width)
        }
        if n.measure_height >= 0.0 && s.size.height.is_auto() {
            s.size.height = Dimension::length(n.measure_height)
        }
        match tree.new_leaf(s) {
            Ok(id) => ids.push(id),
            Err(_) => return -2,
        }
    }
    for (index, _n) in input.iter().enumerate() {
        let children: Vec<NodeId> = input
            .iter()
            .enumerate()
            .filter_map(|(i, c)| (c.parent == index as i32).then_some(ids[i]))
            .collect();
        if tree.set_children(ids[index], &children).is_err() {
            return -3;
        }
    }
    let root_index = input.iter().position(|n| n.parent < 0).unwrap_or(0);
    if tree
        .compute_layout(
            ids[root_index],
            Size {
                width: AvailableSpace::Definite(viewport_width),
                height: AvailableSpace::Definite(viewport_height),
            },
        )
        .is_err()
    {
        return -4;
    }
    for (index, n) in input.iter().enumerate() {
        let Ok(l) = tree.layout(ids[index]) else {
            return -5;
        };
        let mut x = l.location.x;
        let mut y = l.location.y;
        let mut p = n.parent;
        while p >= 0 {
            x += tree.layout(ids[p as usize]).unwrap().location.x;
            y += tree.layout(ids[p as usize]).unwrap().location.y;
            p = input[p as usize].parent
        }
        output[index] = OutputBox {
            id: n.id,
            x,
            y,
            width: l.size.width,
            height: l.size.height,
            content_width: l.scrollable_overflow_rect.right - l.scrollable_overflow_rect.left,
            content_height: l.scrollable_overflow_rect.bottom - l.scrollable_overflow_rect.top,
        }
    }
    count as i32
}
