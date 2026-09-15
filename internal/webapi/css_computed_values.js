let svgComputedTransform = null;
let webAnimationComputedValue = () => undefined;
// Computed values project the cascade, font resources and canonical box graph.
// The catalog supplies initial semantics, never measured element coordinates.
const cssComputedShorthand = (element, name) => {
  const components = cssComputedShorthands[name],
    values = components.map((n) => cssComputedValue(element, n)),
    get = (n) => cssComputedValue(element, n),
    same = values.every((v) => v === values[0]);
  if (values.some((v) => v === '')) return '';
  if (['background-position', 'mask-position', '-webkit-mask-position'].includes(name))
    return values.join(' ');
  if (name === 'border') {
    const sides = ['top', 'right', 'bottom', 'left'].map((side) =>
      ['width', 'style', 'color'].map((part) => get('border-' + side + '-' + part)).join(' '),
    );
    return sides.every((v) => v === sides[0]) ? sides[0] : '';
  }
  if (name === 'rule') {
    const row = get('row-rule'),
      column = get('column-rule');
    return row === column ? row : row + ' / ' + column;
  }
  if (
    name === 'text-decoration' &&
    get('text-decoration-line') === 'none' &&
    get('text-decoration-style') === 'solid' &&
    get('text-decoration-thickness') === 'auto'
  )
    return 'none';
  if (name === '-webkit-mask-box-image' && values.join(' ') === 'none 0 fill auto 0 stretch')
    return 'none';
  if (/^page-break-/.test(name)) return values[0] === 'page' ? 'always' : values[0];
  if (name === 'font')
    return [
      get('font-style') === 'normal' ? '' : get('font-style'),
      get('font-variant-caps') === 'normal' ? '' : get('font-variant-caps'),
      get('font-weight') === '400' ? '' : get('font-weight'),
      get('font-stretch') === '100%' ? '' : get('font-stretch'),
      get('font-size') + (get('line-height') === 'normal' ? '' : ' / ' + get('line-height')),
      get('font-family'),
    ]
      .filter(Boolean)
      .join(' ');
  if (name === 'flex-flow') return values.join(' ');
  if (
    name === 'animation' &&
    get('animation-name') === 'none' &&
    get('animation-duration') === '0s' &&
    get('animation-delay') === '0s' &&
    get('animation-timing-function') === 'ease' &&
    get('animation-iteration-count') === '1' &&
    get('animation-direction') === 'normal' &&
    get('animation-fill-mode') === 'none' &&
    get('animation-play-state') === 'running'
  )
    return 'none';
  if (name === 'background')
    return (
      get('background-color') +
      ' ' +
      get('background-image') +
      ' ' +
      get('background-repeat') +
      ' ' +
      get('background-attachment') +
      ' ' +
      get('background-position') +
      ' / ' +
      get('background-size') +
      ' ' +
      get('background-origin') +
      ' ' +
      get('background-clip')
    );
  if (
    name === 'border-image' &&
    get('border-image-source') === 'none' &&
    get('border-image-slice') === '100%' &&
    get('border-image-width') === '1' &&
    get('border-image-outset') === '0' &&
    get('border-image-repeat') === 'stretch'
  )
    return 'none';
  if (
    name === 'mask' &&
    get('mask-image') === 'none' &&
    get('mask-size') === 'auto' &&
    get('mask-repeat') === 'repeat' &&
    get('mask-origin') === 'border-box' &&
    get('mask-clip') === 'border-box' &&
    get('mask-composite') === 'add' &&
    get('mask-mode') === 'match-source'
  )
    return 'none';
  if (name === 'border-spacing')
    return get('-webkit-border-horizontal-spacing') === get('-webkit-border-vertical-spacing')
      ? get('-webkit-border-horizontal-spacing')
      : get('-webkit-border-horizontal-spacing') + ' ' + get('-webkit-border-vertical-spacing');
  if (name === 'border-block' || name === 'border-inline') {
    const prefix = name + '-start-';
    if (values.slice(0, 3).some((v, i) => v !== values[i + 3])) return '';
    return get(prefix + 'width') + ' ' + get(prefix + 'style') + ' ' + get(prefix + 'color');
  }
  if (name === 'outline')
    return get('outline-color') + ' ' + get('outline-style') + ' ' + get('outline-width');
  if (name === 'list-style')
    return (
      get('list-style-position') + ' ' + get('list-style-image') + ' ' + get('list-style-type')
    );
  if (name === 'font-synthesis')
    return (
      ['weight', 'style', 'small-caps']
        .filter((v) => get('font-synthesis-' + v) === 'auto')
        .join(' ') || 'none'
    );
  if (name === 'white-space') {
    const a = get('white-space-collapse'),
      b = get('text-wrap-mode');
    return a === 'collapse'
      ? b === 'wrap'
        ? 'normal'
        : 'nowrap'
      : a === 'preserve'
        ? b === 'wrap'
          ? 'pre-wrap'
          : 'pre'
        : a === 'preserve-breaks' && b === 'wrap'
          ? 'pre-line'
          : values.join(' ');
  }
  if (name === 'grid') return values.join(' / ');
  if (name === 'grid-template' && same) return values[0];
  if (name === 'offset')
    return (
      get('offset-path') +
      ' ' +
      get('offset-distance') +
      ' ' +
      get('offset-rotate') +
      (get('offset-anchor') === 'auto' ? '' : ' / ' + get('offset-anchor'))
    );
  if (
    [
      'container',
      'position-try',
      'text-wrap',
      'text-box',
      'scroll-timeline',
      'view-timeline',
      'timeline-trigger',
    ].includes(name)
  ) {
    const kept = values.filter((v, i) => v !== cssInitialValues.get(components[i]));
    return kept.length
      ? kept.join(' ')
      : {
          container: 'none',
          'position-try': 'none',
          'text-wrap': 'wrap',
          'text-box': 'normal',
          'scroll-timeline': 'none',
          'view-timeline': 'none',
          'timeline-trigger': 'none',
        }[name];
  }
  if (name === 'row-rule') return values.filter((v, i) => i !== 1 || v !== 'none').join(' ');
  if (same) return values[0];
  if (/^(grid-(area|row|column))$/.test(name)) return values.join(' / ');
  if (values.length === 4 && !/animation|transition/.test(name)) return cssCompressFour(values);
  const ordinary = serializeOrdinaryCSSShorthand(name, values);
  return ordinary || values.join(' ');
};
const cssComputedValue = (element, name) => {
  // Borrowed nodes resolve in their owner. Do not build a caller-side style
  // observation that cannot be retained and never participates in resolution.
  if (styleObservationIsolated) {
    const foreign = foreignCSSObservation(element, 'value', name);
    if (foreign !== null) return foreign;
  }
  return withStyleReadCache(() => {
    const foreign = foreignCSSObservation(element, 'value', name);
    if (foreign !== null) return foreign;
    if (!computedStyleDocumentAvailable(element) || !computedStyleAvailable(element)) return '';
    const initial = cssInitialValues.get(name),
      entries = computedCSSDeclarations(element),
      animated = webAnimationComputedValue(element, name);
    const declaration =
        animated === undefined ? entries.find((e) => e.name === name) : { name, value: animated },
      specified = declaration?.value;
    let value = specified,
      inherit = cssInheritedProperties.has(name);
    if (value === 'inherit' || ((value == null || value === 'unset') && inherit)) {
      value = undefined;
      for (
        let p = cssFontParent(element);
        elementSlot(p)?.type === 'element';
        p = cssFontParent(p)
      ) {
        const v = computedCSSDeclarations(p).find((e) => e.name === name)?.value;
        if (v != null && !['inherit', 'unset'].includes(v)) {
          value =
            v === 'currentcolor' ||
            (name === 'caret-color' && v === 'auto') ||
            (name === 'line-height' && cssNumberRegex.test(v))
              ? v
              : cssComputedValue(p, name);
          break;
        }
      }
      if (value == null) value = initial;
    }
    if (value == null || ['initial', 'unset', 'revert', 'revert-layer'].includes(value))
      value = initial;
    if (cssComputedShorthands[name]) return cssComputedShorthand(element, name);
    if (value === undefined) {
      if (name === 'page') return 'auto';
      if (/^background-position-[xy]$/.test(name)) {
        const pair = (
          entries.find((e) => e.name === 'background-position')?.value || '0% 0%'
        ).split(' ');
        return pair[name.endsWith('x') ? 0 : 1] || '0%';
      }
      return '';
    }
    if (name === 'transform' && value !== 'none') {
      const raw = declaration?.parsedValue ?? value;
      try {
        const svg = svgComputedTransform?.(element, raw);
        if (svg !== null && svg !== undefined) return svg;
        const box = layoutRectFor(element);
        const matrix = compatibilityMatrix.parse(raw, (text, axis) =>
          cssResolveLength(
            text,
            cssGeometryLengthContext(element, axis === 0 ? box.width : axis === 1 ? box.height : 0),
          ),
        );
        const two =
          [2, 3, 6, 7, 8, 9, 11, 14].every((i) => matrix[i] === 0) &&
          matrix[10] === 1 &&
          matrix[15] === 1;
        return (
          (two ? 'matrix(' : 'matrix3d(') +
          (two ? [0, 1, 4, 5, 12, 13].map((i) => matrix[i]) : matrix)
            .map(cssSerializeNumber)
            .join(', ') +
          ')'
        );
      } catch {
        return value;
      }
    }
    if (name === 'color') return cssResolvedColor(element);
    if (
      ['opacity', 'fill-opacity', 'stroke-opacity', 'stop-opacity', 'flood-opacity'].includes(
        name,
      ) &&
      cssNumberRegex.test(value)
    )
      return cssSerializeNumber(Math.max(0, Math.min(1, Number(value))));
    if (value === 'currentcolor' || (name === 'caret-color' && value === 'auto'))
      return cssResolvedColor(element);
    if (/color$/.test(name) || ['fill', 'stroke'].includes(name)) {
      const rgba = cssColorRGBA(value, cssUsedColorScheme(element));
      if (rgba) return cssSerializeColor(rgba);
    }
    if (name === 'font-size') return cssSerializeNumber(cssComputedFontSize(element) ?? 16) + 'px';
    if (
      /^margin-(top|right|bottom|left)$/.test(name) &&
      specified == null &&
      elementSlot(element).tagName === 'BODY'
    )
      return '8px';
    if (name === 'font-weight')
      return value === 'normal' ? '400' : value === 'bold' ? '700' : value;
    if (
      name === 'display' &&
      elementSlot(element).tagName === 'INPUT' &&
      String(host.getAttribute(elementSlot(element).nodeId, 'type') || '').toLowerCase() ===
        'hidden'
    )
      return 'none';
    if (name === 'display') {
      const display = specified == null ? cssBoxModel.state(element).display : value;
      const position = entries.find((e) => e.name === 'position')?.value;
      return ['absolute', 'fixed'].includes(position) ? blockifiedDisplay(display) : display;
    }
    if (
      name === 'unicode-bidi' &&
      specified == null &&
      ['DIV', 'P', 'SECTION', 'ARTICLE', 'HEADER', 'FOOTER', 'MAIN', 'LI', 'TABLE'].includes(
        elementSlot(element).tagName,
      )
    )
      return 'isolate';
    const writing = (() => {
        for (let p = element; elementSlot(p)?.type === 'element'; p = cssFontParent(p)) {
          const v = computedCSSDeclarations(p).find((e) => e.name === 'writing-mode')?.value;
          if (v && !['inherit', 'unset'].includes(v)) return v === 'initial' ? 'horizontal-tb' : v;
        }
        return 'horizontal-tb';
      })(),
      vertical = writing.startsWith('vertical') || writing.startsWith('sideways'),
      rtl = (name === 'direction' ? value : cssComputedValue(element, 'direction')) === 'rtl';
    const sides = vertical
      ? {
          block: {
            start: writing.endsWith('-rl') ? 'right' : 'left',
            end: writing.endsWith('-rl') ? 'left' : 'right',
          },
          inline: { start: rtl ? 'bottom' : 'top', end: rtl ? 'top' : 'bottom' },
        }
      : {
          block: { start: 'top', end: 'bottom' },
          inline: { start: rtl ? 'right' : 'left', end: rtl ? 'left' : 'right' },
        };
    const logical = {
      'block-size': vertical ? 'width' : 'height',
      'inline-size': vertical ? 'height' : 'width',
      'min-block-size': vertical ? 'min-width' : 'min-height',
      'min-inline-size': vertical ? 'min-height' : 'min-width',
      'max-block-size': vertical ? 'max-width' : 'max-height',
      'max-inline-size': vertical ? 'max-height' : 'max-width',
      'inset-block-start': sides.block.start,
      'inset-block-end': sides.block.end,
      'inset-inline-start': sides.inline.start,
      'inset-inline-end': sides.inline.end,
    };
    if (logical[name] && (specified == null || declaration?.allReset || value === 'auto'))
      return cssComputedValue(element, logical[name]);
    const physical = /^(border|padding|margin)-(block|inline)-(start|end)(.*)$/.exec(name);
    if (physical && (specified == null || declaration?.allReset))
      return cssComputedValue(
        element,
        physical[1] + '-' + sides[physical[2]][physical[3]] + physical[4],
      );
    const box = () => cssBoxModel.size(element),
      state = cssBoxModel.state(element),
      length = (v, basis = 0) => cssResolveLength(v, cssGeometryLengthContext(element, basis));
    if (
      /^margin-(left|right)$/.test(name) &&
      value === 'auto' &&
      !['inline', 'inline-block'].includes(state.display)
    )
      return cssSerializeNumber(box().edges[name.endsWith('left') ? 'mleft' : 'mright']) + 'px';
    if (name === 'width' || name === 'height') {
      for (let p = element; p; p = geometryParent(p))
        if (cssBoxModel.state(p).display === 'none') {
          const n = value.endsWith('%') ? null : length(value);
          return n === null ? value : cssSerializeNumber(n) + 'px';
        }
      const dimensions =
          name === 'width' ? cssBoxModel.widthBox(element) : cssBoxModel.heightBox(element),
        dimension = dimensions[name],
        edges = dimensions.edges;
      return (
        cssSerializeNumber(
          Math.max(
            0,
            dimension -
              (state.get('box-sizing') === 'border-box'
                ? 0
                : name === 'width'
                  ? edges.pleft + edges.pright + edges.bleft + edges.bright
                  : edges.ptop + edges.pbottom + edges.btop + edges.bbottom),
          ),
        ) + 'px'
      );
    }
    if (/^min-(width|height)$/.test(name) && value === 'auto') return '0px';
    if (/^(top|right|bottom|left)$/.test(name) && ['fixed', 'absolute'].includes(state.position)) {
      const r = cssBoxModel.rect(element),
        parent = state.position === 'fixed' ? null : geometryParent(element),
        p = parent ? cssBoxModel.rect(parent) : { x: 0, y: 0, ...host.viewport() };
      return (
        cssSerializeNumber(
          name === 'left'
            ? r.x - p.x
            : name === 'top'
              ? r.y - p.y
              : name === 'right'
                ? p.width - r.x + p.x - r.width
                : p.height - r.y + p.y - r.height,
        ) + 'px'
      );
    }
    if (name === 'transform-origin' || name === 'perspective-origin') {
      if (
        state.display === 'inline' &&
        !['absolute', 'fixed'].includes(state.position) &&
        !replacedGeometryTags.has(elementSlot(element).tagName)
      )
        return '0px 0px';
      const words = value.split(/\s+/),
        axis = [box().width, box().height],
        keywords = { left: '0%', top: '0%', center: '50%', right: '100%', bottom: '100%' };
      return words
        .map((v, i) => {
          const n = length(keywords[v] || v, axis[i] || 0);
          return n === null ? v : cssSerializeNumber(n) + 'px';
        })
        .join(' ');
    }
    if (/^border-.*-width$/.test(name)) {
      const style = cssComputedValue(element, name.replace(/width$/, 'style'));
      if (style === 'none' || style === 'hidden') return '0px';
    }
    if (value === 'medium' && /width$/.test(name)) return '3px';
    if (value === 'thin' && /width$/.test(name)) return '1px';
    if (value === 'thick' && /width$/.test(name)) return '5px';
    if (name === 'line-height' && cssNumberRegex.test(value))
      return cssSerializeNumber(Number(value) * (cssComputedFontSize(element) ?? 16)) + 'px';
    if (
      /^(margin|padding)-/.test(name) ||
      /^(min|max)-(width|height)$/.test(name) ||
      /width$/.test(name) ||
      ['line-height', 'letter-spacing', 'word-spacing', 'text-indent', 'outline-offset'].includes(
        name,
      )
    ) {
      const n = length(
        value,
        name === 'line-height'
          ? (cssComputedFontSize(element) ?? 16)
          : geometryParent(element)
            ? cssBoxModel.width(geometryParent(element))
            : 0,
      );
      if (n !== null) return cssSerializeNumber(n) + 'px';
    }
    return value;
  });
};
