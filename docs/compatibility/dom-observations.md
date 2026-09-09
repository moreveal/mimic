# DOM tree observations and inert parsing

Document children and legacy collections read the canonical tree, retain stable
collection identity and stay live after mutations. Plugins aliases embeds;
applets is the empty collection observed in Chrome 152. Node position comparison
uses actual parent/child order, with attribute ordering and stable inverse
implementation-specific ordering for disconnected roots.

DOMParser imports a parsed tree into the existing node arena under its own root.
It creates no Window or event loop and starts no scripts or resources. Ownership
survives removal; inserting a node into another document adopts its subtree
without changing wrapper identity. HTML uses the existing HTML parser. Parsed
documents retain content type, source document URL and their own compatibility
mode. This is shared canonical storage, not a second synchronized DOM model.

XML handles element namespaces, attributes, comments, text and malformed-input
parsererror nodes using a non-networking decoder. Full DTD/entity support,
processing-instruction Web API coverage and Chrome's exact localized error
markup/messages remain limitations. Trusted Types enforcement is not introduced
by this change. These are explicit remaining compatibility boundaries, not
claims that arbitrary XML input is fully Chrome-equivalent.

The retained Chrome oracle covers positions, attributes, collection identity and
liveness, HTML/XML/SVG parsing, inert script content and invalid API calls:
`compatibility/captures/semantic-checkpoints/dom-observations-chrome152.json`.
Additional tests cover adoption, disconnected roots and namespace preservation.
