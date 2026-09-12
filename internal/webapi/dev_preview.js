// Included only in explicitly enabled dev realms. Reads private revisions and
// CSSOM; DOM, shadow roots and live controls use the existing Go snapshot path.
registerBootstrapCallback('installDevPreview',kind=>kind==='version'?
  host.observationVersion()+':'+constructedStyleSheets.revision()+':'+compatibilityElementState.observationVersion()+':'+devPreviewFormRevision+':'+windowScrollX+':'+windowScrollY:
  {width:host.viewport().width,height:host.viewport().height,modals:compatibilityElementState.modalNodes(),styles:constructedStyleSheets.previewSources(document)});
