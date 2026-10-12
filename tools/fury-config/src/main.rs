// SPDX-License-Identifier: AGPL-3.0-or-later
//! fury-config: the persona catalogue and config derivation of Fury
//! (fury-shared), as a small CLI AnyProfile calls. JSON on stdout, errors on
//! stderr, non-zero exit on failure. Never given a secret.
//!
//!   fury-config personas --os macos
//!   fury-config pick --os macos --roll <u64>
//!   fury-config locale --country <CC>
//!   fury-config derive --persona <id> --seed <hex16> --timezone <iana>
//!       --languages <a,b> --ui-locale <tag> [--lat <f> --lon <f>]
//!       --chrome-major <n> --chrome-full <a.b.c.d>

use fury_shared::{catalogue, fingerprint, locale, persona::seed, Persona, ProfileContext};
use serde_json::json;
use std::collections::HashMap;
use std::process::ExitCode;

fn main() -> ExitCode {
    let args: Vec<String> = std::env::args().skip(1).collect();
    match run(&args) {
        Ok(v) => {
            println!("{v}");
            ExitCode::SUCCESS
        }
        Err(e) => {
            eprintln!("fury-config: {e}");
            ExitCode::from(2)
        }
    }
}

fn run(args: &[String]) -> Result<serde_json::Value, String> {
    let (cmd, rest) = args.split_first().ok_or("missing command")?;
    let f = flags(rest)?;
    match cmd.as_str() {
        "personas" => Ok(json!(for_os(f.get("os"))?
            .iter()
            .map(|p| json!({
                "id": p.id, "os": p.os.name, "weight": p.weight,
                "screen": format!("{}x{}", p.screen.width, p.screen.height),
                "gpu": p.gpu.webgl_renderer,
            }))
            .collect::<Vec<_>>())),
        "pick" => {
            let roll: u64 = need(&f, "roll")?.parse().map_err(|_| "--roll must be a u64")?;
            Ok(json!({ "personaId": pick(&for_os(f.get("os"))?, roll).id }))
        }
        "locale" => {
            let l = locale::for_country(need(&f, "country")?).unwrap_or(locale::FALLBACK);
            Ok(json!({ "uiLocale": l.ui, "languages": l.languages }))
        }
        "derive" => derive(&f),
        other => Err(format!("unknown command {other:?}")),
    }
}

fn derive(f: &HashMap<String, String>) -> Result<serde_json::Value, String> {
    let id = need(f, "persona")?;
    let persona = catalogue::all().into_iter().find(|p| p.id == *id).ok_or(format!("unknown persona {id:?}"))?;
    let seed = seed::from_hex(need(f, "seed")?).ok_or("--seed must be 16 hex digits")?;
    let geolocation = match (f.get("lat"), f.get("lon")) {
        (Some(a), Some(b)) => Some((a.parse().map_err(|_| "bad --lat")?, b.parse().map_err(|_| "bad --lon")?)),
        (None, None) => None,
        _ => return Err("--lat and --lon go together".into()),
    };
    let ctx = ProfileContext {
        timezone: need(f, "timezone")?.clone(),
        languages: need(f, "languages")?.split(',').map(str::to_string).collect(),
        ui_locale: need(f, "ui-locale")?.clone(),
        geolocation,
        chrome_major: need(f, "chrome-major")?.parse().map_err(|_| "bad --chrome-major")?,
        chrome_full_version: need(f, "chrome-full")?.clone(),
    };
    let cfg = persona.derive_core_config(seed, &ctx);
    fingerprint::check_core_config(&cfg).map_err(|e| format!("config check failed: {}", e.join("; ")))?;
    Ok(cfg)
}

/// Personas for one OS family ("macos", "windows"); macOS is the only one
/// AnyProfile offers on a Mac host, since a foreign OS leaks through voices.
fn for_os(os: Option<&String>) -> Result<Vec<Persona>, String> {
    let want = match os.map(String::as_str) {
        Some("macos") => "macOS",
        Some("windows") => "Windows",
        other => return Err(format!("--os must be macos or windows, got {other:?}")),
    };
    Ok(catalogue::all().into_iter().filter(|p| p.os.name == want).collect())
}

/// Weighted choice within the filtered set, the same draw as catalogue::pick_weighted.
fn pick(set: &[Persona], roll: u64) -> &Persona {
    let total: f64 = set.iter().map(|p| p.weight).sum();
    let mut cursor = (roll >> 11) as f64 / (1u64 << 53) as f64 * total;
    for p in set {
        cursor -= p.weight;
        if cursor < 0.0 {
            return p;
        }
    }
    set.last().expect("catalogue has personas for this OS")
}

fn flags(rest: &[String]) -> Result<HashMap<String, String>, String> {
    let mut out = HashMap::new();
    let mut it = rest.iter();
    while let Some(k) = it.next() {
        let key = k.strip_prefix("--").ok_or(format!("unexpected argument {k:?}"))?;
        let v = it.next().ok_or(format!("--{key} needs a value"))?;
        out.insert(key.to_string(), v.clone());
    }
    Ok(out)
}

fn need<'a>(f: &'a HashMap<String, String>, k: &str) -> Result<&'a String, String> {
    f.get(k).ok_or(format!("missing --{k}"))
}
