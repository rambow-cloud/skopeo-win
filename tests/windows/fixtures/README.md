# Signature integrity fixture

`image-uncompressed.signature` contains the original signed OpenPGP packets from `cmd/skopeo/fixtures/image.signature`, with only the outer compression packet unwrapped. Its key, manifest and identity are the existing upstream fixtures. No private key is included here.

The 728-byte stream consists of a one-pass signature packet, literal data packet and signature packet. It was derived by parsing the outer `packet.Compressed` object with the existing ProtonMail packet reader and reading its body. The baseline executable test must verify it successfully before rejection cases run.

Integrity tests mutate the actual signature packet, not the original compressed stream's trailing bytes: changing only compression framing can leave every authenticated packet unchanged. Git text conversion is disabled to preserve exact bytes.
