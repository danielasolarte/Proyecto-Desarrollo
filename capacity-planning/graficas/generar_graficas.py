"""Graficas del informe de pruebas de carga de la Entrega 3.

Lee los resultados crudos del repositorio (k6 JSON y CSV de los monitores) y
escribe los PNG en esta misma carpeta. Desde la raiz del repositorio:

    python capacity-planning/graficas/generar_graficas.py

Requiere matplotlib (pip install matplotlib).
"""
import csv
import json
from datetime import datetime, timedelta, timezone
from pathlib import Path

import matplotlib

matplotlib.use("Agg")
import matplotlib.dates as mdates  # noqa: E402
import matplotlib.pyplot as plt  # noqa: E402

RAIZ = Path(__file__).resolve().parents[2]
SALIDA = Path(__file__).resolve().parent
RES = RAIZ / "capacity-planning" / "results"

# Paleta categorica (orden fijo) y tinta de texto/ejes.
AZUL, NARANJA, AQUA = "#2a78d6", "#eb6834", "#1baf7a"
TINTA, TINTA2, REJILLA, FONDO = "#1f1f1e", "#5f5e58", "#e4e3df", "#fcfcfb"

plt.rcParams.update({
    "figure.facecolor": FONDO, "axes.facecolor": FONDO, "savefig.facecolor": FONDO,
    "axes.edgecolor": REJILLA, "axes.labelcolor": TINTA2, "text.color": TINTA,
    "xtick.color": TINTA2, "ytick.color": TINTA2, "axes.grid": True,
    "grid.color": REJILLA, "grid.linewidth": 0.8, "axes.axisbelow": True,
    "axes.spines.top": False, "axes.spines.right": False,
    "font.size": 10, "axes.titlesize": 11, "axes.titleweight": "bold",
    "legend.frameon": False, "lines.linewidth": 2,
})


def k6(ruta):
    return json.loads(Path(ruta).read_text(encoding="utf-8"))["metrics"]


def guardar(fig, nombre):
    fig.tight_layout()
    fig.savefig(SALIDA / nombre, dpi=150)
    plt.close(fig)
    print("escrita", nombre)


def utc(texto):
    return datetime.fromisoformat(texto.strip('"').replace("Z", "+00:00"))


def barras_agrupadas(ax, etiquetas, series, fmt):
    """series: lista de (nombre, color, valores). Barras con 2 px de separacion."""
    n = len(series)
    ancho = 0.8 / n
    for i, (nombre, color, valores) in enumerate(series):
        xs = [x + (i - (n - 1) / 2) * ancho for x in range(len(etiquetas))]
        barras = ax.bar(xs, valores, ancho, color=color, label=nombre,
                        edgecolor=FONDO, linewidth=1.5)
        ax.bar_label(barras, labels=[fmt(v) for v in valores], padding=2,
                     fontsize=8, color=TINTA2)
    ax.set_xticks(range(len(etiquetas)), etiquetas)
    ax.grid(axis="x", visible=False)


# 1. Escenario 1: p95 y errores, Entrega 2 vs Entrega 3 ------------------------
def escenario1():
    niveles = [10, 25, 50, 100]
    p95 = {"e2": [], "e3": []}
    err = {"e2": [], "e3": []}
    for v in niveles:
        for clave, pref in (("e2", "e1-gcp"), ("e3", "e1-e3")):
            m = k6(RAIZ / "k6" / "results" / f"{pref}-{v}.json")
            p95[clave].append(m["http_req_duration"]["p(95)"])
            err[clave].append(m["http_req_failed"]["value"] * 100)
    etiquetas = [f"{v} VUs" for v in niveles]
    fig, (a1, a2) = plt.subplots(1, 2, figsize=(10, 3.8))
    barras_agrupadas(a1, etiquetas, [("Entrega 2", NARANJA, p95["e2"]),
                                     ("Entrega 3", AZUL, p95["e3"])],
                     lambda v: f"{v:,.0f}".replace(",", " "))
    a1.set_title("Escenario 1: latencia p95 (ms)")
    a1.set_ylabel("ms")
    a1.legend(loc="upper left")
    barras_agrupadas(a2, etiquetas, [("Entrega 2", NARANJA, err["e2"]),
                                     ("Entrega 3", AZUL, err["e3"])],
                     lambda v: f"{v:.2f} %".replace(".", ","))
    a2.set_title("Escenario 1: errores HTTP (%)")
    a2.set_ylabel("%")
    a2.legend(loc="upper left")
    guardar(fig, "e1-p95-errores.png")


# 2. Escenario 2: upload -> ready por nivel ------------------------------------
def escenario2():
    niveles = ["baseline", "l1", "l2", "l3"]
    prom, p95 = [], []
    for n in niveles:
        m = k6(RAIZ / "docs" / "entrega3" / "evidencias-escenario2" / f"escenario2-e3-{n}.json")
        prom.append(m["upload_time_to_ready_ms"]["avg"] / 1000)
        p95.append(m["upload_time_to_ready_ms"]["p(95)"] / 1000)
    fig, ax = plt.subplots(figsize=(7, 3.8))
    barras_agrupadas(ax, ["Baseline", "L1", "L2", "L3"],
                     [("Promedio", AQUA, prom), ("p95", AZUL, p95)],
                     lambda v: f"{v:.0f} s")
    ax.set_title("Escenario 2 (E3): tiempo upload → ready")
    ax.set_ylabel("segundos")
    ax.legend(loc="upper left")
    guardar(fig, "e2-upload-ready.png")


# 3. Escenario 3A: CPU por instancia y tamano del grupo ------------------------
def escenario3a():
    cpu = {}
    with open(RES / "e3-3a-cpu-20261031-133449.csv", encoding="utf-8-sig") as f:
        for fila in csv.DictReader(f):
            cpu.setdefault(fila["instancia"], []).append((utc(fila["utc"]), float(fila["cpu_pct"])))
    t_grupo, grupo, sanas = [], [], []
    with open(RES / "e3-3a-timeline-20261031-124530.csv", encoding="utf-8-sig") as f:
        for fila in csv.DictReader(f):
            t_grupo.append(utc(fila["utc"]))
            grupo.append(int(fila["tamano_objetivo"]))
            sanas.append(len([s for s in fila["sanas_lb"].split() if s.strip()]))
    inicio, fin = t_grupo[0], t_grupo[-1]

    fig, (a1, a2) = plt.subplots(2, 1, figsize=(10, 6), sharex=True,
                                 gridspec_kw={"height_ratios": [3, 2]})
    colores = [AZUL, NARANJA, AQUA]
    for color, (inst, puntos) in zip(colores, sorted(cpu.items())):
        puntos = [p for p in puntos if inicio <= p[0] <= fin]
        a1.plot([p[0] for p in puntos], [p[1] for p in puntos], color=color,
                label=inst, marker="o", markersize=3)
    a1.axhline(60, color=TINTA2, linestyle="--", linewidth=1)
    a1.text(inicio, 61.5, "objetivo 60 %", color=TINTA2, fontsize=8)
    a1.set_ylabel("CPU (%)")
    a1.set_ylim(0, 100)
    a1.set_title("Escenario 3A: CPU por instancia (promedio por minuto)")
    a1.legend(loc="upper right", ncol=3)

    a2.step(t_grupo, grupo, where="post", color=AZUL, label="Tamaño objetivo del grupo")
    a2.step(t_grupo, sanas, where="post", color=NARANJA, linestyle="--",
            label="Instancias sanas en el balanceador")
    a2.set_ylim(0, 3.6)
    a2.set_yticks([0, 1, 2, 3])
    a2.set_ylabel("instancias")
    a2.set_title("Tamaño del grupo mooc-api-mig")
    a2.legend(loc="upper right")
    a2.xaxis.set_major_formatter(mdates.DateFormatter("%H:%M", tz=timezone.utc))
    a2.set_xlabel("hora UTC, 31/10/2026")
    guardar(fig, "e3a-cpu-grupo.png")


# 4. Escenario 3A: reparto durante la exclusion --------------------------------
def exclusion():
    filas = []
    with open(RES / "e3-3a-exclusion-20261031-125402.csv", encoding="utf-8-sig") as f:
        for fila in csv.DictReader(f):
            if fila["instancia"]:
                filas.append((utc(fila["utc"]), fila["instancia"]))
    instancias = sorted({i for _, i in filas})
    t0 = filas[0][0]
    cubos = {}
    for t, inst in filas:
        cubo = int((t - t0).total_seconds() // 5)
        cubos.setdefault(cubo, {}).setdefault(inst, 0)
        cubos[cubo][inst] += 1
    xs = sorted(cubos)
    fig, ax = plt.subplots(figsize=(10, 3.6))
    for color, inst in zip([AZUL, NARANJA], instancias):
        ax.step([t0 + timedelta(seconds=5 * x) for x in xs],
                [cubos[x].get(inst, 0) for x in xs], where="post", color=color, label=inst)
    bloqueo = datetime(2026, 10, 31, 17, 54, 36, tzinfo=timezone.utc)
    desbloqueo = datetime(2026, 10, 31, 17, 55, 53, tzinfo=timezone.utc)
    ax.axvspan(bloqueo, desbloqueo, color=REJILLA, alpha=0.7, linewidth=0)
    ax.text(bloqueo, ax.get_ylim()[1] * 0.92, " Redis bloqueado en 9gm4", fontsize=8, color=TINTA2)
    ax.set_ylabel("respuestas cada 5 s")
    ax.set_title("Escenario 3A: instancia que responde durante la exclusión (todas HTTP 200)")
    ax.legend(loc="upper right")
    ax.xaxis.set_major_formatter(mdates.DateFormatter("%H:%M:%S", tz=timezone.utc))
    ax.set_xlabel("hora UTC, 31/10/2026")
    guardar(fig, "e3a-exclusion.png")


# 5. Escenario 3B: demanda de la cola ------------------------------------------
def escenario3b():
    # Muestras del exportador de la cola durante la primera carga peak
    # (tabla de la seccion 7.4 del informe; 29/10/2026, UTC).
    muestras = [("03:34:33", 1, 0, 1), ("03:35:33", 7, 3, 4), ("03:36:03", 6, 2, 4),
                ("03:36:33", 8, 4, 4), ("03:37:03", 8, 2, 6), ("03:38:03", 7, 3, 4),
                ("03:39:03", 4, 0, 4), ("03:40:03", 1, 0, 1), ("03:40:33", 0, 0, 0)]
    t = [datetime.strptime("2026-10-29 " + m[0], "%Y-%m-%d %H:%M:%S").replace(tzinfo=timezone.utc)
         for m in muestras]
    fig, ax = plt.subplots(figsize=(9, 3.8))
    ax.plot(t, [m[1] for m in muestras], color=AZUL, marker="o", markersize=4,
            label="Demanda (pendientes + activos)")
    ax.plot(t, [m[3] for m in muestras], color=AQUA, marker="o", markersize=4, label="Activos")
    ax.plot(t, [m[2] for m in muestras], color=NARANJA, marker="o", markersize=4, label="Pendientes")
    alta = datetime(2026, 10, 29, 3, 36, 28, tzinfo=timezone.utc)
    ax.axvline(alta, color=TINTA2, linestyle="--", linewidth=1)
    ax.text(alta, 8.6, " insert mooc-worker-zp1h", fontsize=8, color=TINTA2)
    ax.set_ylim(0, 9.5)
    ax.set_ylabel("trabajos")
    ax.set_title("Escenario 3B: cola media durante la carga peak")
    ax.legend(loc="upper right")
    ax.xaxis.set_major_formatter(mdates.DateFormatter("%H:%M", tz=timezone.utc))
    ax.set_xlabel("hora UTC, 29/10/2026")
    guardar(fig, "e3b-cola.png")


if __name__ == "__main__":
    escenario1()
    escenario2()
    escenario3a()
    exclusion()
    escenario3b()
