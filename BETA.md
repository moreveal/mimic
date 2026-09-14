# Public Beta

**[Download Mimic for Windows or Linux →](https://github.com/moreveal/mimic-overview/releases/tag/v0.1.0-beta.2)**

The beta is publicly downloadable. No Discord invitation is required.
Start with the [quick start](QUICKSTART.md) or the [runnable examples](examples/README.md).

| Platform | Requirements |
| --- | --- |
| Windows amd64 | Verified on Windows 11; extract and run `mimic.exe`. |
| Linux amd64 | glibc 2.39+, libgcc_s, installed Liberation/DejaVu/Noto fonts; verified on Ubuntu 24.04 under WSL2. |

No Chrome, Go, Rust, display server, or GPU is needed for the executable.
Node.js 22+ is needed for the bundled example clients. ARM64 and musl are not
packaged. Read [release notes](RELEASE_NOTES.md) for supported workflows and limitations.

## Feedback and workflow help

**DM `moreveal` on Discord**, or open a public issue with:

- The Mimic release, operating system, and automation client version.
- What you expected and what happened.
- A minimal reproducer using public or local test data.
- Relevant error output, with secrets removed.

Keep credentials, cookies, tokens, and private customer data out of public issues.
For sensitive reproduction details, contact `moreveal` first.

## License

Mimic uses [PolyForm Shield 1.0.0](LICENSE.md). Commercial use is permitted
subject to the license's noncompete provisions and other conditions. Client
examples are MIT licensed; bundled dependencies retain their own terms.
The beta remains in active development and does not promise complete browser compatibility.
