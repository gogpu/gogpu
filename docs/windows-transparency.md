# Windows Transparent Windows

## Two presentation paths

On Windows there are two different ways a transparent gogpu window can be
composited:

### 1. DirectComposition (DX12, explicit)

`WS_EX_NOREDIRECTIONBITMAP` is set at `CreateWindowExW` time and
`DwmEnableBlurBehindWindow` is **not** called. wgpu creates a
`CreateSwapChainForComposition` swap chain and presents through a DComp
visual tree. This is the only path on which a DX12 swap chain's per-pixel
alpha composites directly against the desktop.

### 2. Legacy DWM blur-behind (Vulkan / GLES / Auto)

The window keeps the DWM redirection surface and
`DwmEnableBlurBehindWindow(DWM_BB_ENABLE | DWM_BB_BLURREGION)` is called with
an empty region. The GPU swap chain presents through the HWND and DWM uses the
alpha channel of the redirection surface. This works for Vulkan (including
`VK_COMPOSITE_ALPHA_OPAQUE_BIT_KHR`, because DWM-level blur handles alpha) and
GLES.

`WS_EX_NOREDIRECTIONBITMAP` must not be used on this path: Vulkan/GLES/Software
present through the HWND, and without a redirection surface the window content
is invisible.

## Verified matrix

Verified on Windows 11 with an NVIDIA GeForce RTX 4060 (driver 591.86) using a
frameless 50% transparent red panel over a known backdrop:

| API | Window style | 50% alpha result |
|-----|--------------|------------------|
| DX12 (explicit) | `WS_EX_NOREDIRECTIONBITMAP` + DComp | per-pixel alpha visible |
| Vulkan (explicit) | legacy blur-behind | per-pixel alpha visible |
| GLES (explicit) | legacy blur-behind | per-pixel alpha visible |
| Software (explicit) | legacy blur-behind | **not supported** — GDI `BitBlt`/`StretchDIBits` (`SRCCOPY`) does not preserve alpha |
| Auto | legacy blur-behind | visible, but see the DX12 caveat below |

## Known limitations

### Software backend

The software backend presents through GDI `BitBlt`/`StretchDIBits` with
`SRCCOPY`, which does not carry per-pixel alpha. `DwmEnableBlurBehindWindow`
alone is not enough. A future implementation should use
`UpdateLayeredWindow` (or `AlphaBlend` from a DIB section) for transparent
software windows. Until then, `WithTransparent(true)` + Software should be
treated as unsupported.

### Auto + DX12

Auto mode deliberately uses the legacy blur-behind path because the backend is
not known at window creation time and Auto prefers Vulkan. If wgpu selects DX12
on a Vulkan-less machine, the DX12 backend still creates a DirectComposition
swap chain whenever `CompositeAlphaModePremultiplied` is requested, but without
`WS_EX_NOREDIRECTIONBITMAP` the DComp content composites against the opaque
redirection surface instead of the desktop. The result is a white/opaque tint,
not per-pixel transparency.

Options:

- Require callers who need true DX12 transparency to set
  `WithGraphicsAPI(GraphicsAPIDX12)` explicitly.
- Resolve the backend before window creation (larger refactor).
- Recreate the window when Auto resolves to DX12 (Godot's approach). This
  destroys and recreates the HWND, so it can cause a brief visible flash /
  focus change; doing it before the first `Show` minimizes the visible effect.
