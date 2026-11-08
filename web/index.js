import Module from "./rl/raylib.js";
import { Runtime } from "./runtime.js";

const canvas = document.getElementById("canvas");
canvas.addEventListener("contextmenu", (e) => e.preventDefault());

// Compile the Go program while raylib downloads and initialises.
const mainModule = WebAssembly.compileStreaming(fetch("main.wasm"));

// Emscripten locates ./rl/raylib.wasm next to raylib.js on its own.
const raylib = await Module({ canvas });

const go = new Go();
go.importObject.raylib = raylib;
globalThis.raylib = raylib; // read by the Runtime helpers
globalThis.mod = raylib; // read by wasm-ffi-go: js.Global().Get("mod")

// Helpers the Go side imports via go:wasmimport gojs
const runtime = new Runtime();
Object.assign(go.importObject.gojs, {
  CStringFromGoString: runtime.CStringFromGoString,
  CStringGetLength: runtime.CStringGetLength,
  CStringArrayGetLength: runtime.CStringArrayGetLength,
  CopyToC: runtime.CopyToC,
  CopyToGo: runtime.CopyToGo,
  Alert: runtime.Alert,
});

const instance = await WebAssembly.instantiate(await mainModule, go.importObject);
globalThis.goInstance = instance; // read by the Runtime helpers
go.run(instance);
