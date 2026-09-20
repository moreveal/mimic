use blitz_dom::{DocumentConfig, StyleThreading};
use blitz_html::HtmlDocument;
use blitz_traits::shell::{ColorScheme, Viewport};
use std::hint::black_box;
use std::time::{Duration, Instant};

const ROWS: usize = 10_000;

fn fixture() -> String {
    let mut html = String::with_capacity(400_000);
    html.push_str("<!doctype html><meta charset=utf-8><style>html,body{margin:0;padding:0}body{font:16px/16px Arial}.row{height:2px}#target{display:block;width:100%;height:32px;padding:0;border:0}</style><body><section id=fixture style=\"width:800px\"><button id=target type=button>Probe</button>");
    for i in 0..ROWS {
        html.push_str("<div class=row data-i=");
        html.push_str(&i.to_string());
        html.push_str("></div>");
    }
    html.push_str("</section></body>");
    html
}

fn timed<T>(f: impl FnOnce() -> T) -> (T, Duration) {
    let start = Instant::now();
    let value = f();
    (value, start.elapsed())
}

fn main() {
    let html = fixture();
    let threading = if std::env::var_os("BLITZ_SEQUENTIAL").is_some() {
        StyleThreading::Sequential
    } else {
        StyleThreading::Parallel
    };
    let config = DocumentConfig {
        viewport: Some(Viewport::new(1280, 800, 1.0, ColorScheme::Light)),
        style_threading: threading,
        ..Default::default()
    };

    let (mut doc, parse) = timed(|| HtmlDocument::from_html(&html, config));
    let (_, first_resolve) = timed(|| doc.resolve(0.0));
    let target = doc.query_selector("#target").unwrap().unwrap();
    let node = doc.get_node(target).unwrap();
    let first_layout = node.final_layout();
    let first_pos = node.absolute_position(0.0, 0.0);
    let display = doc.resolved_style_value(target, "display");
    assert!((first_layout.size.width - 800.0).abs() < 0.1, "{first_layout:?}");
    assert!((first_layout.size.height - 32.0).abs() < 0.1, "{first_layout:?}");
    assert_eq!(display, "block");

    let (_, reuse_reads) = timed(|| {
        for _ in 0..10_000 {
            let node = doc.get_node(target).unwrap();
            black_box(node.final_layout());
            black_box(node.absolute_position(0.0, 0.0));
            black_box(doc.resolved_style_value(target, "display"));
        }
    });

    let (_, no_change_resolve) = timed(|| doc.resolve(0.0));

    let (_, target_mutation) = timed(|| {
        {
            let mut mutation = doc.mutate();
            mutation.set_attribute(
                target,
                blitz_dom::qual_name!("style"),
                "display:block;width:640px;height:40px;padding:0;border:0",
            );
        }
        doc.resolve(0.0);
    });
    let mutated_layout = doc.get_node(target).unwrap().final_layout();
    assert!((mutated_layout.size.width - 640.0).abs() < 0.1, "{mutated_layout:?}");
    assert!((mutated_layout.size.height - 40.0).abs() < 0.1, "{mutated_layout:?}");

    doc.set_viewport(Viewport::new(1279, 800, 1.0, ColorScheme::Light));
    let (_, viewport_rebuild) = timed(|| doc.resolve(0.0));
    let rebuilt = doc.get_node(target).unwrap();
    let rebuilt_layout = rebuilt.final_layout();
    let rebuilt_pos = rebuilt.absolute_position(0.0, 0.0);
    assert!((rebuilt_layout.size.width - 640.0).abs() < 0.1, "{rebuilt_layout:?}");

    let center = (
        rebuilt_pos.x + rebuilt_layout.size.width / 2.0,
        rebuilt_pos.y + rebuilt_layout.size.height / 2.0,
    );
    let (_, hit_tests) = timed(|| {
        for _ in 0..10_000 {
            black_box(doc.hit(center.0, center.1));
        }
    });

    println!("rows={ROWS}");
    println!("style_threading={threading:?}");
    println!("html_bytes={}", html.len());
    println!("parse_projection_ms={:.3}", parse.as_secs_f64() * 1000.0);
    println!("first_resolve_ms={:.3}", first_resolve.as_secs_f64() * 1000.0);
    println!("first_build_total_ms={:.3}", (parse + first_resolve).as_secs_f64() * 1000.0);
    println!("reuse_10k_style_rect_ms={:.3}", reuse_reads.as_secs_f64() * 1000.0);
    println!("no_change_resolve_ms={:.3}", no_change_resolve.as_secs_f64() * 1000.0);
    println!("target_style_mutation_resolve_ms={:.3}", target_mutation.as_secs_f64() * 1000.0);
    println!("viewport_rebuild_ms={:.3}", viewport_rebuild.as_secs_f64() * 1000.0);
    println!("hit_test_10k_ms={:.3}", hit_tests.as_secs_f64() * 1000.0);
    println!("target_rect=({}, {}) {}x{}", first_pos.x, first_pos.y, rebuilt_layout.size.width, rebuilt_layout.size.height);
}
