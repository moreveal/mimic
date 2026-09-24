window.probe = {
  read(value) {
    return `${value}:${document.querySelectorAll('#items li').length}:${window.location.pathname}`;
  },
  async callDotNet(reference, value) {
    await reference.invokeMethodAsync('Receive', value);
  },
};
