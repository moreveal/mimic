//! Canonical FontFaceSet resources supplied by the embedding Page.
use std::sync::Arc;
use linebender_resource_handle::Blob;
use parley::fontique::{Collection, CollectionOptions, FontInfoOverride, FontStyle, FontWeight, SourceCache};
use crate::BaseDocument;

pub struct CanonicalFont {
    pub family: String,
    pub weight: f32,
    pub italic: bool,
    pub bytes: Vec<u8>,
}

impl BaseDocument {
    /// Replace, rather than append, so FontFaceSet.delete/clear remove coverage.
    /// Preserve the shared context identity held by Stylo's device.
    pub fn replace_canonical_fonts(&mut self, fonts: Vec<CanonicalFont>) {
        let mut replacement = parley::FontContext {
            source_cache: SourceCache::new_shared(),
            collection: Collection::new(CollectionOptions {
                shared: false,
                system_fonts: cfg!(all(feature = "system-fonts", not(target_arch = "wasm32"))),
            }),
        };
        replacement.collection.register_fonts(Blob::new(Arc::new(crate::BULLET_FONT) as _), None);
        for font in fonts {
            let info = FontInfoOverride {
                family_name: Some(&font.family),
                weight: Some(FontWeight::new(font.weight)),
                style: Some(if font.italic { FontStyle::Italic } else { FontStyle::Normal }),
                ..Default::default()
            };
            replacement.collection.register_fonts(Blob::new(Arc::new(font.bytes)), Some(info));
        }
        *self.font_ctx.lock().unwrap() = replacement;
        #[cfg(feature = "parallel-construct")]
        self.thread_font_contexts.clear();
        self.invalidate_inline_contexts();
        // Font-relative computed lengths (ch/ex and metric-dependent values)
        // must be recalculated as well as line shaping after collection edits.
        self.stylist.force_stylesheet_origins_dirty(style::stylesheets::OriginSet::all());
    }
}
