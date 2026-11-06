# Vendored compatibility patches

`image-windows-archive-paths.patch` preserves Windows archive-path support in
the image dependency. After `go mod vendor`, run this from the repository root
unless the new dependency already provides an equivalent fix:

```powershell
git apply hack/vendor-patches/image-windows-archive-paths.patch
```

Run the Windows executable archive integration scenarios after a dependency refresh.
