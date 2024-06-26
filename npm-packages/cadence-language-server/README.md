# Cadence Language Server

The [Cadence](https://github.com/onflow/cadence) language server compiled to WebAssembly and bundled as an NPM package,
so it can be used in tools written in JavaScript.


## Releasing

To release a new version of the Language server NPM package all you need to do is create a release of Langauge server and GitHub action will also publish a new version of WebAssembly built binary to NPM.
That newly build NPM package using the WebAssembly will be published and can be found on NPM https://www.npmjs.com/package/@onflow/cadence-language-server

## Development

### Updating `src/go.js`

- Copy `misc/wasm/wasm_exec.js` of appropriate Go version into `src/go.js`.
- Run `npx prettier -w src/go.js`
- Restore the first line (adopted from header) and last line (`export const go = new Go();`)
- Update the version in the header
- Remove the anonymous function wrapper
- Change `globalThis.Go = class {` to `class Go {`
