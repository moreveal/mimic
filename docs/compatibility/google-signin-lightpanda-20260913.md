# Google sign-in: Mimic and Lightpanda observation

Date: 2026-09-13. Fresh owned processes, native browser identities, no existing
profiles, no password. Opened https://www.google.com/, activated its account
link, focused the identifier, used CDP Input.insertText with the same reserved
address `mimic-compatibility-probe@example.invalid`, and activated Next with
the DOM button click method. This is not a trusted pointer-event comparison.

| Runtime | Google-selected flow | Observed result after submission |
| --- | --- | --- |
| Mimic committed optimization build | GlifWebSignIn | `/v3/signin/rejected`: «Возможно, этот браузер или приложение небезопасны.» |
| Lightpanda default | WebLiteSignIn | Account not found: `ეს ანგარიში ვერ მოიძებნა`; remains on identifier page |
| Lightpanda with additional resources and CORS | WebLiteSignIn | CAPTCHA prompt appears; `/Captcha` request recorded; remains on identifier page |

Mimic reports Chrome/152 in its native User-Agent; Lightpanda reports
Lightpanda/1.0. Google also selected Russian versus Georgian language and
different home-page markup. The reason for flow selection or rejection was
not isolated. Lightpanda submits a document POST to `/v3/signin/identifier`;
Mimic uses AccountsSignInUi RPCs. Therefore the absence of the rejection in
these Lightpanda trials does not establish equivalent application execution
or successful authentication. Additional resources were iframe, stylesheet,
worker and image, with experimental CORS enabled. One submitted trial per
configuration; the later CAPTCHA cannot be attributed to these flags alone.
No CAPTCHA was solved and no real account was tested.

The first Lightpanda exploration used the richer form's nested-button selector
and did not submit. Those two observations are excluded. The corrected helper
supports the directly identified submit button in WebLiteSignIn and checks
evaluation errors. A preliminary Mimic run stopped before sign-in due to local
console encoding and is also excluded. Fixed 7/12/12-second observation waits
were used; these runs are not navigation latency benchmarks.

## Versions and evidence

- Mimic `.build/mimic-optimized.exe`, commit `fd9079c4312335ba01f966c97cdad228b728a9a6`,
  SHA-256 `3a4e6af7093c9a53ec232dc5ad7ff49c7f2fe663307c4838ad6b1edd0de3fdf0`.
- Lightpanda Ubuntu WSL `1.0.0-nightly.9268+909108e29`, SHA-256
  `eb50fc78e575a99ad9b667ffdf7891b188f083f6d93c8411ac33d8809bed1d8b`.
- Local helper: `.build/google_signin_lightpanda_compare.py`.
- Private summaries and CDP events, under `compatibility/private-captures/`:
  `google-signin-lp-compare-mimic-1789303918`,
  `google-signin-lp-compare-lightpanda-1789304023`,
  `google-signin-lp-compare-lightpanda-full-1789304042`.

Session URLs and cookies remain in private captures. No production code was
modified by this comparison. Existing independent compatibility work in the
checkout was left untouched; the tested Mimic binary is identified above.
