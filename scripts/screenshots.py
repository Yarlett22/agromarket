"""Captura pantallas de la demo (usado en CI): login con Keycloak, compra, WAF y vistas principales."""
import time
from playwright.sync_api import sync_playwright

GW = "https://localhost:8443"

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

    m = b.new_context(viewport={"width": 390, "height": 844}, is_mobile=True, ignore_https_errors=True).new_page()
    m.goto(GW)
    m.wait_for_selector(".card")
    m.screenshot(path="shots/7-movil.png")
    b.close()
print("screenshots ok")
