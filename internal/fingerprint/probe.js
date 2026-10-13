// SPDX-License-Identifier: AGPL-3.0-or-later
(async () => {
  const c = document.createElement('canvas'); c.width = 280; c.height = 60;
  const x = c.getContext('2d'); x.textBaseline = 'top'; x.font = '16px Arial';
  x.fillStyle = '#f60'; x.fillRect(100, 1, 62, 20);
  x.fillStyle = '#069'; x.fillText('AnyProfile fp 1.0', 2, 15);
  x.fillStyle = 'rgba(102,204,0,0.7)'; x.fillText('AnyProfile fp 1.0', 4, 17);
  const hash = async (s) => Array.from(new Uint8Array(await crypto.subtle.digest('SHA-256',
    new TextEncoder().encode(s)))).map((b) => b.toString(16).padStart(2, '0')).join('').slice(0, 16);
  const gl = document.createElement('canvas').getContext('webgl');
  const dbg = gl && gl.getExtension('WEBGL_debug_renderer_info');
  return { canvas: await hash(c.toDataURL()),
    webglVendor: dbg ? gl.getParameter(dbg.UNMASKED_VENDOR_WEBGL) : '',
    webglRenderer: dbg ? gl.getParameter(dbg.UNMASKED_RENDERER_WEBGL) : '',
    ua: navigator.userAgent, platform: navigator.platform,
    hwc: navigator.hardwareConcurrency, mem: navigator.deviceMemory,
    screen: screen.width + 'x' + screen.height,
    tz: Intl.DateTimeFormat().resolvedOptions().timeZone, langs: navigator.languages.join(','),
    secure: isSecureContext };
})()
