(() => {
  document.body.innerHTML =
    '<script>secret()</script><div>A<span style="display:none">X</span><br>B</div><p>C <span>D</span></p><div style="visibility:hidden">Y</div>';
  const detached = document.createElement('div');
  detached.innerHTML = '<script>detached()</script><span>E</span>';
  return {
    body: document.body.innerText,
    detached: detached.innerText,
    script: document.querySelector('script').innerText,
  };
})()
