# Taste (Continuously Learned by [CommandCode][cmd])

[cmd]: https://commandcode.ai/

# workflow
- Commit por feature y haz push después de cada feature completada. Confidence: 0.85
- Documenta todo con calidad enterprise — este es un proyecto open source. Confidence: 0.85
- CLI-first antes de web UI. No agregar frontend web hasta que el core CLI esté sólido. Confidence: 0.70

# architecture
- Paper trading primero con arquitectura preparada para conectar cualquier exchange real después. No implementar APIs de exchange real en fases iniciales. Confidence: 0.70
- Aplicar patrones de diseño de software (GoF, enterprise patterns) combinados — ej: si usás Strategy/Factory, reemplazá los switches con magic strings por un registry/dispatch map tipado. Confidence: 0.70
- Vertical slicing real: cada feature (trading, backtest, mcp) contiene su propio domain + app + delivery en un solo directorio. No separar en layers horizontales (cli/, domain/, trading/) — los tipos compartidos van en shared/. Cada feature exporta su handler y cmd/greedy/main.go los cablea. Confidence: 0.85

# testing
- Escribe tests realistas con edge cases, no solo happy path. Confidence: 0.85

