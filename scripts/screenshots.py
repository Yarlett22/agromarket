"""Captura pantallas de la demo (usado en CI): login con Keycloak, compra, WAF y vistas principales."""
import html
import os
import time
from playwright.sync_api import sync_playwright

GW = "https://localhost:8443"

TERM_CSS = """
body{margin:0;background:#1f3d2b;font-family:'DejaVu Sans Mono',Consolas,monospace}
.win{margin:28px;border-radius:12px;overflow:hidden;box-shadow:0 12px 40px rgba(0,0,0,.45);background:#0f1a14}
.bar{background:#2b3a31;color:#cfd8cf;font:600 15px 'DejaVu Sans',sans-serif;padding:10px 16px;display:flex;gap:8px;align-items:center}
.dot{width:12px;height:12px;border-radius:50%;display:inline-block}
pre{margin:0;padding:18px 22px;color:#e6e1d3;font-size:17px;line-height:1.45;white-space:pre-wrap;word-break:break-all}
.cmd{color:#f0a36b}.ok{color:#8fd19e}.bad{color:#ff8a7a}.hl{color:#f6d38a}
"""


def colorize(line):
    e = html.escape(line)
    if line.startswith("$ "):
        return f'<span class="cmd">{e}</span>'
    if "✔" in line or "✅" in line or " t " in line or "True" in line:
        return f'<span class="ok">{e}</span>'
    if "✖" in line or "403" in line or "Access denied" in line:
        return f'<span class="bad">{e}</span>'
    if line.startswith("▶"):
        return f'<span class="hl">{e}</span>'
    return e


def terminal(page, title, blocks, path, width=1366):
    """Renderiza en una ventana de terminal la salida REAL capturada en el CI."""
    body = []
    for cmd, text in blocks:
        body.append(colorize("$ " + cmd))
        body += [colorize(l) for l in text.rstrip("\n").split("\n")]
        body.append("")
    doc = (f"<html><head><meta charset='utf-8'><style>{TERM_CSS}</style></head><body><div class='win'>"
           f"<div class='bar'><span class='dot' style='background:#ff5f56'></span><span class='dot' style='background:#ffbd2e'>"
           f"</span><span class='dot' style='background:#27c93f'></span>&nbsp; {html.escape(title)}</div>"
           f"<pre>{chr(10).join(body)}</pre></div></body></html>")
    page.set_viewport_size({"width": width, "height": 400})
    page.set_content(doc)
    page.locator(".win").screenshot(path=path)


def read(name):
    f = os.path.join("shots", name)
    return open(f, encoding="utf-8").read() if os.path.exists(f) else "(sin salida)"

with sync_playwright() as p:
    b = p.chromium.launch()
    ctx = b.new_context(viewport={"width": 1366, "height": 900}, ignore_https_errors=True)  # CA interna
    pg = ctx.new_page()
    pg.goto(GW)
    pg.wait_for_selector(".card")
    pg.screenshot(path="shots/1-catalogo.png", full_page=True)

    # Login en Keycloak (Authorization Code + PKCE)
    pg.click("header button:has-text('Ingresar')")
    pg.wait_for_selector("#username", timeout=30000)
    pg.fill("#username", "demo")
    pg.fill("#password", "demo1234")
    pg.screenshot(path="shots/2-login-keycloak.png")
    pg.click("#kc-login")
    pg.wait_for_selector(".user", timeout=30000)

    cards = pg.locator(".card")
    cards.nth(0).locator("button:has-text('Agregar')").click()
    cards.nth(0).locator(".qty button:has-text('+')").click()
    cards.nth(1).locator("button:has-text('Agregar')").click()
    pg.click(".cart-btn")
    pg.screenshot(path="shots/3-carrito.png")
    pg.click("button:has-text('Confirmar pedido')")
    pg.wait_for_selector(".order")
    time.sleep(0.6)
    pg.screenshot(path="shots/4-pedido-procesando.png", full_page=True)
    pg.wait_for_selector(".badge.confirmado", timeout=20000)
    pg.screenshot(path="shots/5-pedido-confirmado.png", full_page=True)

    pg.click("nav button:has-text('Arquitectura')")
    pg.wait_for_selector(".layers")
    time.sleep(2.5)
    pg.screenshot(path="shots/6-arquitectura.png", full_page=True)

    # Ataque de inyección SQL bloqueado por el WAF
    w = ctx.new_page()
    w.set_viewport_size({"width": 1000, "height": 260})
    w.goto(GW + "/api/catalog/products?q=1'%20OR%20'1'='1")
    w.screenshot(path="shots/8-waf-bloqueo.png")

    # ---------- Evidencias de terminal (salida real del CI) ----------
    t = ctx.new_page()
    terminal(t, "Terminal — contenedores en ejecución", [("docker compose ps", read("term-ps.txt"))], "shots/9-terminal-contenedores.png")
    terminal(t, "Terminal — prueba end-to-end", [("./scripts/smoke-test.sh", read("term-smoke.txt"))], "shots/10-terminal-pruebas.png")
    terminal(t, "Terminal — TLS interno", [
        ('docker compose exec postgres psql -c "select pid, ssl, version, cipher from pg_stat_ssl where ssl"', read("term-pg.txt")),
        ("curl http://localhost:8222/varz   # monitor del Event Bus NATS", read("term-nats.txt")),
    ], "shots/11-terminal-tls.png")
    attack = GW + "/api/catalog/products?q=1'%20OR%20'1'='1"
    r = ctx.request.get(attack)
    terminal(t, "Terminal — ataque de inyección SQL bloqueado por el WAF", [
        ("curl -k \"https://localhost:8443/api/catalog/products?q=1' OR '1'='1\"", f"HTTP {r.status}\n{r.text()}"),
        ("docker compose logs gateway | grep ModSecurity", read("term-waf.txt")),
    ], "shots/12-terminal-waf.png")
    tok = ctx.request.post(GW + "/auth/realms/agromarket/protocol/openid-connect/token", form={
        "grant_type": "password", "client_id": "agromarket-web", "username": "demo", "password": "demo1234"}).json()["access_token"]
    import base64, json as _j
    hdr = _j.loads(base64.urlsafe_b64decode(tok.split(".")[0] + "=="))
    pl = _j.loads(base64.urlsafe_b64decode(tok.split(".")[1] + "=" * (-len(tok.split(".")[1]) % 4)))
    shown = {k: pl[k] for k in ["iss", "sub", "azp", "preferred_username", "email", "realm_access", "exp"] if k in pl}
    me = ctx.request.get(GW + "/api/users/me", headers={"Authorization": "Bearer " + tok})
    noauth = ctx.request.get(GW + "/api/orders")
    terminal(t, "Terminal — token JWT RS256 emitido por Keycloak", [
        ("curl -k -X POST https://localhost:8443/auth/realms/agromarket/protocol/openid-connect/token -d username=demo ...",
         "header : " + _j.dumps(hdr) + "\npayload: " + _j.dumps(shown, ensure_ascii=False, indent=2)),
        ("curl -k https://localhost:8443/api/orders            # sin token", f"HTTP {noauth.status}\n{noauth.text()}"),
        ("curl -k -H \"Authorization: Bearer <token>\" https://localhost:8443/api/users/me", f"HTTP {me.status}\n{me.text()}"),
    ], "shots/13-terminal-jwt.png")

    # ---------- Consola de administración de Keycloak ----------
    try:
        a = ctx.new_page()
        a.set_viewport_size({"width": 1366, "height": 800})
        a.goto(GW + "/auth/admin/")
        a.wait_for_selector("#username", timeout=30000)
        a.fill("#username", "admin")
        a.fill("#password", "admin")
        a.click("#kc-login")
        a.wait_for_load_state("networkidle", timeout=30000)
        a.goto(GW + "/auth/admin/master/console/#/agromarket/users")
        a.wait_for_selector("text=profesor", timeout=40000)
        time.sleep(2)
        a.screenshot(path="shots/14-keycloak-admin.png")
        a.goto(GW + "/auth/admin/master/console/#/agromarket/clients")
        a.wait_for_selector("text=agromarket-web", timeout=40000)
        time.sleep(2)
        a.screenshot(path="shots/15-keycloak-clientes.png")
    except Exception as e:  # la consola es opcional para el CI
        print("consola Keycloak:", e)

    m = b.new_context(viewport={"width": 390, "height": 844}, is_mobile=True, ignore_https_errors=True).new_page()
    m.goto(GW)
    m.wait_for_selector(".card")
    m.screenshot(path="shots/7-movil.png")
    b.close()
print("screenshots ok")
