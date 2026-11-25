# Code signing (optional)

Conduit releases are published unsigned. Windows SmartScreen may warn on first run and after an in-app update.

## Sign with signtool

1. Get an Authenticode certificate (EV gives instant SmartScreen reputation).
2. Install it in the Windows certificate store, or keep a `.pfx` file.
3. Sign every binary you publish:

```powershell
signtool sign /fd SHA256 /tr http://timestamp.digicert.com /f cert.pfx /p PASSWORD build\conduit.exe
signtool sign /fd SHA256 /tr http://timestamp.digicert.com /f cert.pfx /p PASSWORD build\conduit-updater.exe
signtool sign /fd SHA256 /tr http://timestamp.digicert.com /f cert.pfx /p PASSWORD build\Conduit-Setup-1.3.0.exe
```

4. Verify:

```powershell
signtool verify /pa build\conduit.exe
```

Do not change the binary after signing: the updater verifies `conduit.exe.sha256`, so re-signing must happen before checksums are generated.

## CI

`.github/workflows/release.yml` has a commented optional signing step gated on `secrets.WINDOWS_CERT_BASE64`. Decode the PFX only on protected branches and never commit certificates.
