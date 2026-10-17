"""Captura pantallas de la demo (usado en CI): registro, compra y vistas principales."""
import time
from playwright.sync_api import sync_playwright

GW = "http://localhost:8080"

with sync_playwright() as p:
    b = p.chromium.launch()
    pg = b.new_page(viewport={"width": 1366, "height": 900})
    pg.goto(GW)
    pg.wait_for_selector(".card")
    pg.screenshot(path="shots/1-catalogo.png", full_page=True)

    pg.click("text=Ingresar")
    pg.fill("form.modal label:has-text('Nombre') input", "Exportadora Demo SAC")
    pg.fill("input[type=email]", f"demo{int(time.time())}@agromarket.pe")
    pg.fill("input[type=password]", "demo1234")
    pg.screenshot(path="shots/2-registro.png")
    pg.click("button:has-text('Registrarme')")
    pg.wait_for_selector(".user")

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

    m = b.new_page(viewport={"width": 390, "height": 844}, is_mobile=True)
    m.goto(GW)
    m.wait_for_selector(".card")
    m.screenshot(path="shots/7-movil.png")
    b.close()
print("screenshots ok")
