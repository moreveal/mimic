(() => {
  const read = (scheme, color) => {
    const canvas = document.createElement('canvas');
    canvas.width = canvas.height = 1;
    canvas.style.colorScheme = scheme;
    document.body.append(canvas);
    const context = canvas.getContext('2d');
    context.fillStyle = color;
    context.fillRect(0, 0, 1, 1);
    const result = [context.fillStyle, ...context.getImageData(0, 0, 1, 1).data];
    canvas.remove();
    return result;
  };
  return {
    lightCanvas: read('light', 'Canvas'),
    darkCanvas: read('dark', 'Canvas'),
    lightText: read('light', 'CanvasText'),
    darkText: read('dark', 'CanvasText'),
  };
})();
