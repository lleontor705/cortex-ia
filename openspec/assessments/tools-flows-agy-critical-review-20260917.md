# Auditoría crítica de herramientas, flujos y AGY

Fecha: 2026-09-17. Base Cortex-IA: d1e5c35 con cambios locales. Fuente Cortex instalada comprobada mediante `go version -m`: v2.3.9, disponible en `C:/Users/usrLuisLeon/go/pkg/mod/github.com/lleontor705/cortex/v2@v2.3.9`. Evaluación de diseño, no contrato de implementación ni evidencia de cambios nuevos. No se consultaron secretos ni se ejecutaron trabajos AGY reales durante esta auditoría.

## Conclusión

Los hashes son necesarios en varias fronteras internas; trasladar su cálculo o transcripción al modelo suele ser innecesario. `cortex_cortex_relate` tiene valor real: conecta observaciones que después consumen búsqueda y navegación. No debe exigirse mecánicamente en cada revisión. La prioridad es corregir contratos desfasados, comprobar preparación antes de aceptar trabajos y reducir la interfaz visible por objetivo y capacidades reales.

## Hallazgos prioritarios

1. **P1 — Autenticación de AGY incompatible con el entorno inspeccionado.** El aislamiento exige `CORTEX_IA_AGY_AUTH=gemini` y `GEMINI_API_KEY`; ambas faltan en los ámbitos inspeccionados. La configuración de cuenta por defecto no equivale a autenticación probada. El puente acepta el trabajo antes de esa comprobación; el worker puede registrar running y luego fallar sin iniciar AGY. Ver `internal/delegation/execution_environment.go:22`, `internal/delegation/runner.go:247`, `internal/delegation/runner.go:483`, `internal/assets/plugins/herdr-bridge.ts:1417`. La corrección propuesta es preparación previa a la aceptación, revalidación al ejecutar y mecanismos de autenticación compatibles explícitos. No copiar perfiles ni prometer acceso al keyring con HOME temporal sin comprobarlo.
2. **P1 — Instrucciones contradicen el esquema instalado.** `internal/assets/plugins/cortex.ts:311` dice que get_blast_radius solo acepta observaciones numéricas. Cortex v2.3.9 tiene `target` para símbolos/archivos y un argumento legacy de observación; remoto utiliza `node_id`. La selección de rama depende del argumento: un observation_id entero usa observaciones; target usa grafo de código. Fuente C:342 y C:1303. Generar ayuda desde capacidades/versiones reales evita duplicar esquemas en prosa.
3. **P1 — Disponibilidad dependiente del transporte.** get_project_context/list_skills/get_skill/graph_subgraph están registrados en el servidor remoto, no en el registro MCP local inspeccionado. Un inicio obligatorio que los exija en local es inviable. detect_cycles sí existe en ambos backends; que no aparezca en esta sesión no significa que no esté implementado. Fuentes H:1595, H:1600, H:1608 y C:370.
4. **P2 — Exceso de ceremonia manual.** reviewer.md:110 exige recuperar contenido y volver a pasarlo al hash; reviewer.md:162 y :173 exige relate tanto en FAIL como PASS. Snapshot local ya recupera y calcula/verifica el digest en una operación. La aprobación transaccional ya compara fingerprints. Mantener la evidencia en backend, devolver verificaciones compactas y exigir relación solo cuando exista una relación semántica concreta.
5. **P2 — El catálogo no equivale al inventario visible.** CORTEX_TOOLS es una lista utilizada por el plugin para contabilidad, no un registro de capacidades. El perfil del backend y los permisos del rol filtran posteriormente. Inferir disponibilidad desde esa lista provoca llamadas imposibles. Fuente `internal/assets/plugins/cortex.ts:203` y S:30.
6. **P1 — Utilidades pueden escribir sin reserva.** doc_convert permite output arbitrario y está autorizado para reviewer/orchestrator; el destino pasa a MkdirAll/WriteFile sin lease. La prueba del plugin real con proceso simulado confirmó que reviewer puede transmitir un destino exterior no reservado; no se ejecutó una sobrescritura real. Fuente B:881, `internal/app/doc.go:88`, `internal/docconv/docconv.go:248`. diagram_render presenta el mismo patrón por inspección (`internal/diagram/render.go:57`, :98), sin probe separado. El lease guard solo intercepta nombres de edición convencionales (`cortex-lease-guard.ts:108`). Separar extracción de lectura de exportación autorizada y validar todos los destinos en el servicio.
7. **P1 — Entrega descarta evidencia declarada.** work_transition declara summary/verdict/evidence_refs/changed_files pero no los transmite al comando. El probe real con mock confirmó la omisión. No implica que la aprobación pueda saltarse, pero sí que la entrega pierde información que el esquema promete. Fuente B:1255–1285. Alinear esquema y persistencia; verificar recuperación de esos campos desde estado/eventos.
8. **P2 — Recuperación y contexto demasiado globales.** work_recover no tiene scope y el servicio puede recuperar revisiones de más de 30 minutos (`internal/delegation/work.go:970`); riesgo de afectar una revisión ajena identificado por código, no incidente reproducido. La compactación inyecta work list global sin límites por board/proyecto (B:764). Acotar selección y conservar CAS/TTL, no eliminar recuperación.

## Cómo leer el inventario

Cada fila corresponde a una capacidad canónica. Todos los nombres llevan prefijo `cortex_`; la variante `cortex_cortex_*` es el nombre con prefijo del servidor y no se cuenta otra vez. Ocho parejas de nombres funcionales llaman al mismo handler en el backend local y se agrupan en una fila. Los 63 nombres del catálogo del plugin se reducen así a **55 capacidades**. Sumadas a las 37 herramientas Cortex-IA, se evalúan **92 capacidades del catálogo revisado**, contrastando implementaciones locales y remotas: no son 92 herramientas cargadas simultáneamente ni una captura actual de tools/list. De las 55 de Cortex, 28 nombres canónicos están permitidos en al menos uno de los seis roles; permitir no garantiza disponibilidad del transporte. No se ha medido latencia ni coste de tokens: los costes siguientes son estructurales.

Evidencia abreviada: **M** = fuente Cortex `internal/mcp/tools_memory.go`; **C** = `internal/mcp/tools_cortex.go`; **S** = `internal/mcp/server.go`; **T** = `internal/mcp/temporal_tools.go`; **H** = `internal/platform/server/http.go`. Todas relativas a la raíz exacta v2.3.9 indicada arriba. **R** = permitido por algún rol; **—** = reconocido por el plugin, pero no permitido explícitamente por los seis roles. Veredictos son propuestas, no eliminación ya realizada. «Retirar» se refiere a la superficie ordinaria del agente, preservando administración cuando corresponda.

| Capacidad canónica; alias funcional | Roles | Beneficio concreto | Coste o límite | Veredicto | Evidencia |
|---|---|---|---|---|---|
| search | R | Recuperar evidencia por consulta/filtros | Expansión añade consultas y contexto | Mantener como entrada de búsqueda | M:124; búsqueda:592 |
| save | R | Conservar una decisión o hallazgo reutilizable | Ruido si todo cierre se convierte en memoria | Mantener selectivamente | M:34 |
| update | — | Corregir una observación existente | Puede cambiar evidencia mutable | Opcional para curación | M:281 |
| delete | — | Borrar memoria errónea o innecesaria | Destructivo | Retirar de agentes ordinarios | M:407; S:77 |
| suggest_topic_key | — | Sugerir identidad estable de un tema | Otra llamada para una operación de guardado | Internalizar en save | M:316 |
| save_prompt | — | Conservar un prompt cuando su historial importa | Duplicación, contenido sensible y volumen | Opcional, no automático | M:253 |
| session_summary | R | Resumen de continuidad | Se solapa con handoff/context | Fusionar en cierre de sesión | M:182 |
| context | R | Recuperar contexto reciente | Puede repetir project_dna/agent_context | Mantener una entrada de recuperación | M:158 |
| stats | — | Diagnosticar tamaño/estado de memoria | No resuelve el objetivo del usuario | Internalizar en diagnóstico | M:391; S:78 |
| timeline | R | Reconstruir hechos próximos a un incidente | Contexto innecesario fuera de forense; perfil admin local | Opcional en investigación | M:427; S:79 |
| get_observation | R | Leer evidencia completa y referenciable | Resultado grande si se repite | Mantener; integrar pin/verificación | M:237 |
| session_start | R | Registrar sesión raíz | Llamadas repetidas por subagentes duplican ciclo | Internalizar en host/orquestador | M:341 |
| session_end | R | Cerrar sesión raíz | No sustituye respuesta al usuario | Internalizar en host/orquestador | M:368 |
| capture_passive | — | Extraer aprendizajes desde secciones de texto | Heurística de formato, posible duplicación | Internalizar; opt-in | M:469 |
| relate | R | Vincular causa, antecedente, referencia o contradicción | Dos IDs y juicio semántico; aristas arbitrarias añaden ruido | Mantener condicional | C:34; C:589 |
| graph | R | Navegar observaciones relacionadas | Recorrido puede crecer; necesita límites | Opcional, o expansión de búsqueda | C:67; C:653 |
| graph_relationships | — | Inspeccionar aristas de una observación | Duplica navegación genérica | Fusionar como vista de graph | C:91 |
| graph_path | — | Explicar conexión entre dos observaciones | Búsqueda multisal­to costosa | Opcional forense | C:106 |
| graph_subgraph | — | Subgrafo heterogéneo acotado | Solo registro remoto observado | Opcional visualización/diagnóstico | H:1595 |
| score | R | Inspeccionar importancia asignada | Puntuación no prueba verdad ni calidad | Internalizar ranking; opcional diagnóstico | C:125 |
| archive | — | Retirar memoria conservando historial | Confundible con archive de trabajo SDD | Retirar de superficie ordinaria | C:144; S:80 |
| search_hybrid | R | Combinar señales semánticas y grafo | Solapa search/resolve_query; embeddings y consultas adicionales | Fusionar mediante estrategia interna | C:160; C:780 |
| get_blast_radius; code_impact | R | Impacto de símbolo/archivo o de observación | Esquemas local/remoto distintos; grafo puede estar obsoleto | Mantener con target tipado y nombre claro | C:342; C:1303; H:1596 |
| analyze_architecture; code_analyze | R | Comunidades, concentración y ciclos | Vista global excesiva para un cambio pequeño | Opcional de arquitectura | C:384 |
| detect_cycles | R | Detectar ciclos del grafo indexado | No equivale a fallo de compilación ni a ciclo nuevo | Mantener para cambios estructurales | C:370; C:1397 |
| ingest_code; code_scan | R | Construir índice de símbolos/relaciones | Escaneo y escritura; no exigir en toda lectura | Internalizar actualización incremental | C:319; C:1253 |
| get_code_symbols; code_symbols | R | Consultar símbolos filtrados | Solapa code_find; depende del índice | Fusionar consultas estructurales | C:402 |
| get_code_graph; code_graph | R | Extraer grafo estructural completo | Volumen alto; normalmente basta un subconjunto | Opcional, acotado | C:435 |
| code_map; get_code_map | R | Mapa de repositorio con presupuesto | Duplica contexto si se inyecta y se pide otra vez | Mantener para descubrimiento | C:453 |
| code_tests; get_impacted_tests | R | Encontrar pruebas relacionadas al cambio | Grafo incompleto puede omitir pruebas | Mantener como recomendación, no oráculo único | C:474 |
| code_find; find_symbols | R | Buscar símbolos por texto/kind | Solapa get_code_symbols y búsqueda local | Fusionar con consulta de símbolos | C:499 |
| get_agent_context | — | Paquete de reglas, decisiones y gotchas | Solapa context/project_dna/project_context | Fusionar entrada de contexto | C:534 |
| get_rules | R | Obtener directrices aprobadas | Relecturas reiteradas | Mantener con caché/versionado | C:274 |
| save_rule | — | Cambiar reglas de gobernanza | No es guardar un hallazgo corriente | Retirar de agentes ordinarios | C:292 |
| get_project_context | R | Gobernanza y habilidades del proyecto | Remoto; obligación local no ejecutable | Fusionar contexto, según capacidades | H:1608 |
| list_skills | R | Descubrir habilidades remotas | Repite inventario ya inyectado si existe | Opcional al descubrir/cambiar proyecto | H:1609 |
| get_skill | — | Obtener instrucciones de habilidad remota | Prompt lo exige pero ningún rol lo permite explícitamente | Mantener bajo carga selectiva autorizada | H:1610; cortex.ts:292 |
| resolve_query | R | Consulta unificada según modo | Duplica search y context; más recuperación | Fusionar entrada de búsqueda/contexto | C:252; H:1611 |
| get_status | R | Diagnóstico de modo y capacidades | Ceremonia si se repite por llamada | Internalizar handshake, opcional diagnóstico | C:558 |
| revision_history | R | Auditar cambios en evidencia | Solo necesario si importa evolución | Opcional | M:449 |
| consolidate | — | Detectar candidatos con mismo topic_key | Es sugerencia, no fusión automática | Opcional mantenimiento | C:217 |
| project_dna | — | Resumir conocimiento del proyecto | No comprueba ingestión AST | Fusionar contexto; opcional onboarding | C:236 |
| merge_projects | — | Corregir proyectos duplicados | Mutación administrativa de identidad | Retirar de superficie ordinaria | C:570; S:81 |
| handoff | R | Transferir conocimiento duradero estructurado | No transfiere claims ni sustituye dispatch | Mantener solo cuando cambia responsable/contexto | M:99 |
| temporal_create_edge | — | Registrar validez temporal de relación | Carga semántica extra y perfil especializado | Opcional temporal | T:40 |
| temporal_get_edges | — | Consultar relaciones válidas en una fecha | Requiere necesidad histórica concreta | Opcional temporal | T:41 |
| temporal_get_relevant | — | Recuperar evidencia relevante en fecha | Solapa búsqueda temporal | Fusionar búsqueda con as_of | T:42 |
| temporal_create_snapshot | — | Capturar grafo en un momento | No es pin de contrato; coste de persistencia | Opcional administrativo | T:43 |
| temporal_record_operation | — | Registrar métricas de operación | Agente no debe autodeclarar telemetría mecánica | Internalizar | T:44 |
| temporal_evaluate_quality | — | Evaluar calidad de memoria | Evaluación no prueba tarea correcta | Opcional evaluación, fuera del flujo normal | T:45 |
| temporal_system_metrics | — | Consultar rendimiento agregado | No necesario en una implementación común | Internalizar diagnóstico | T:46 |
| temporal_health_check | — | Estado de subsistema temporal | Solapa get_status | Fusionar diagnóstico | T:47 |
| temporal_evolution_path | — | Seguir evolución de una arista | Especializado, potencial volumen | Opcional forense | T:48 |
| temporal_fact_state | — | Consultar estado actual de un hecho | Solapa lectura enriquecida de observación | Fusionar con get_observation | T:49 |
| search_temporal | — | Buscar evidencia a fecha | Solapa search con as_of | Fusionar estrategia de búsqueda | C:191; C:950 |

### Visibilidad y alias

Roles, sin contar nombres `cortex_ia_*`: discovery permite get_rules, get_status, get_project_context, list_skills, get_observation, search, get_code_symbols, get_code_graph, analyze_architecture y detect_cycles. Implement permite get_rules, get_observation, get_code_symbols, context, save, code_tests y code_find. Investigate añade búsqueda, historia y grafo; planner añade planificación de conocimiento/handoff; reviewer añade verificación/relaciones; orchestrator concentra sesión/contexto. Los nombres exactos están en `internal/assets/agents/{discovery,implement,investigate,planner,reviewer,orchestrator}.md:17`.

Alias funcionales comprobados en C:319–530: code_scan→ingest_code; code_impact→get_blast_radius; code_analyze→analyze_architecture; code_symbols→get_code_symbols; code_graph→get_code_graph; get_code_map→code_map; get_impacted_tests→code_tests; find_symbols→code_find. No es necesario presentar ambos al modelo. Mantener compatibilidad de transporte no exige duplicar documentación ni elección del agente.

El perfil local agent separa herramientas ordinarias de los perfiles admin y temporal; registro `all` también admite alias. El servidor remoto tiene otra composición. Este inventario describe el código, no una captura de tools/list de la sesión actual. La herramienta disponible en un turno debe comprobarse contra el inventario efectivo, sin bloquear por un nombre solo mencionado en instrucciones.

## `relate`: utilidad probada y límite

La ruta local es C:589 → `internal/domain/graph/graph.go:163` → `internal/store/graph/store.go:57`. Escribe aristas entre observaciones, con tipo, peso, confianza y explicación. No escribe `code_relations`; las relaciones AST son producidas por ingestión. Los consumidores reales son navegación (`graph`, relaciones y caminos), `internal/store/search/store.go:592` y `:1099` para añadir vecinos a búsqueda cuando graph_expand=true, y C:854 para ajustar ranking híbrido mediante conexiones. Ese último fragmento ajusta resultados existentes, por lo que no demuestra por sí solo recuperación de resultados nuevos.

Recomendación: conservarlo cuando se pueda afirmar «esta observación documenta la causa de aquella», «sustituye aquella decisión» o una referencia verificable. Evitar `relates_to` indiscriminado y enlaces obligatorios al finalizar. Preferir que `save` admita relaciones explícitas en la misma operación cuando ya se conocen los IDs, manteniendo validación transaccional. No se ha medido mejora de recuperación en Cortex: existencia de consumidores no es un benchmark.

## Auditoría de hashes

| Familia | Protección y necesidad | Qué no demuestra | Simplificación propuesta | Evidencia |
|---|---|---|---|---|
| Tokens claim/lease | Comparar credenciales sin persistir su texto; mantener | No protege contra quien controla proceso/BD ni valida calidad del trabajo | Cálculo exclusivo en servicio; jamás copiar tokens/hash al prompt | `internal/delegation/work.go:137`, :575, :680 |
| Backup/restore | Detectar bytes alterados respecto al manifiesto; mantener | No da procedencia si atacante puede sustituir también manifiesto; no prueba coherencia SQLite de una copia bruta | Backend verifica automáticamente; presentar resultado y ruta | `internal/backup/verify.go:14`, :75; `restore.go:102` |
| Ownership/plan de instalación | Detectar drift y planes desactualizados antes de sobrescribir | No decide intención del usuario, como evidenció Context7 | Mantener digest, resolver selección declarativa por separado | `internal/install/util.go:32`; `service.go:83` |
| Pins SDD | Fijar bytes exactos del contrato y detectar modificación | No demuestra que la especificación sea correcta ni vigente semánticamente | Recuperar+verificar por ID en backend; no retranscribir documentos | `internal/delegation/spec_contract.go:139`, :538; `cortex-convention.md:49` |
| Fingerprint revisión | Vincular aprobación a definición y archivos concretos; mantener | No prueba ejecución de tests, no incluye archivos fuera del scope, no elimina TOCTOU | Verificación integrada en approve/archive; lectura manual solo al diagnosticar drift | `internal/delegation/spec_contract.go:144`, :193 |
| HMAC de reportes | Autenticidad/integridad respecto al secreto compartido; mantener | No prueba verdad del incidente, identidad individual ni recepción remota | Firmar y validar internamente, versionar payload; no manualhash | `internal/telemetry/report.go:186`, :201 |
| Hash de AST | Identidad de bytes e insumo para invalidar caché; mantener interno | No prueba índice completo ni dependencias correctas | Actualizar cuando cambia contenido; no exigir ingestión universal | Cortex `internal/domain/ast/ast.go:219`; `resolver.go:51` |
| content_hash público | Calcula digest exacto de contenido entregado | No autentica origen y es vulnerable a transcripción distinta del contenido esperado | Internalizar o dejar como utilidad excepcional | `internal/assets/plugins/herdr-bridge.ts:780` |
| snapshot_read | Recupera observación local y verifica hash sin transcripción | No demuestra equivalencia con observación remota del mismo ID | Mantener como read_verified; evitar exportar todo proyecto para una lectura | `internal/assets/plugins/cortex-snapshot.ts:8`, :22 |

El extractor inspeccionado calcula hashes por archivo y los incorpora a símbolos; `handleIngestCode` ejecuta ExtractCodeGraph y SaveSymbols/SaveRelations. No se encontró en esa ruta MCP una comparación de hashes previos que garantice «delta <50ms». No presentar esa promesa de skills como propiedad verificada. La caché tampoco debe confundirse con firma criptográfica: un hash sin secreto detecta igualdad respecto a una referencia confiable, no autentica por sí solo.

## AGY: estado real y diseño propuesto

El investigador confirmó AGY 1.2.5 responde a help/version. Hay ejecuciones históricas exitosas, pero no una prueba E2E actual. Implement/investigate/reviewer delegan con skip_permissions=true; planner está nativo. `runner.go:410` aplica el flag solicitado. La autorización del usuario de ese flag se conserva; no es la causa del fallo de preparación.

El proveedor ausente en settings indica el camino predeterminado de cuenta, no una sesión autenticada demostrada. La [documentación oficial de instalación AGY](https://antigravity.google/docs/cli/install/) describe alternativas de autenticación; el entorno temporal actual solo admite Gemini API y además rechaza modelos no Gemini (`runner.go:435`). Esta reducción de compatibilidad debe ser explícita, no presentarse como delegación general lista para usar. No copiar credenciales, ni forzar proveedor/modelo como reparación silenciosa.

Se trata de una regresión de compatibilidad introducida por el endurecimiento previo del entorno, no solo de una configuración externa incompatible. En la ruta examinada, la ausencia de autenticación termina normalmente en `failed/AGY_FAILED` con mensaje `AGY_AUTH_REQUIRED`, no en `blocked`. CompleteWorker limpia la lease del job y ese fallo deja de bloquear el workspace; las claims/leases de la tarea no se liberan ni su estado se transiciona automáticamente. Las pruebas de helper y entorno sintético pasaron, pero no validaron RunWorker con autenticación real ni demostraron una ejecución E2E actual.

Flujo recomendado: configuración → capacidad/autenticación/preparación del servicio → aceptación durable → worker revalida → ejecución → confirmación de terminación → verificación independiente. La validación previa no garantiza que credenciales sigan vigentes, por eso se conserva la revalidación. Si ya hubo aceptación, no ejecutar el mismo objetivo nativamente como fallback automático. Fallos de preparación deben conservar categoría propia; hoy auth puede terminar clasificado como process_exit (`runner.go:286–299`, remoteFailureClass), perdiendo diagnóstico.

## Fundamentación y criterios de mejora

[Intelligent AI Delegation](https://arxiv.org/abs/2602.11865) sustenta separar autoridad, responsabilidad y supervisión: aquí significa comprobar preparación antes de transferir ejecución y conservar aprobación independiente. No demuestra eficacia del producto concreto.

[Writing tools for agents](https://www.anthropic.com/engineering/writing-tools-for-agents) favorece herramientas ajustadas a tareas y evaluadas, no una réplica de cada endpoint. Aplicación propuesta: unificar búsqueda/contexto y operaciones atómicas, conservando límites de autoridad.

[Trabajo sobre selección adaptable de herramientas](https://arxiv.org/abs/2605.24660) motiva evaluar una lista visible según tarea y capacidades. Sus resultados no son mediciones de Cortex; aquí se propone una evaluación local antes de atribuir beneficios.

Criterios futuros medibles: cero llamadas obligatorias a herramientas ausentes en perfiles local/remoto; cero discrepancias esquema/prompt en fixtures por versión; rechazo de autenticación no preparada antes de crear job; preservación de fresh authority y no-fallback después de aceptar; cero transcripciones de contenido solo para calcular hash cuando hay lectura verificada; llamadas/contexto/latencia por flujo comparados contra baseline, junto a tasa de éxito y detección de permisos indebidos. Reducir llamadas no debe ocultar fallos ni reducir verificación sustantiva.

## Inventario completo Cortex-IA

La revisión independiente de bridge_guard instanció el plugin real con loader y mocks de SDK/archivos/procesos y analizó los seis frontmatters con YAML estándar: **36 herramientas del puente y una de snapshot, total 37**. Visibilidad Cortex-IA: orchestrator 21, discovery 12, investigate 18, planner 21, implement 22, reviewer 18. No incluye herramientas nativas ni alias MCP Cortex. No encontró permisos exactos Cortex-IA sin implementación. Los denies de familia seguidos por allows exactos funcionan; conservar esa protección, incluidos los alias de prefijo. B significa `internal/assets/plugins/herdr-bridge.ts`.

| Herramienta | Beneficio concreto | Coste o límite | Veredicto propuesto | Evidencia |
|---|---|---|---|---|
| cortex_ia_content_hash | Digest exacto para pins | Cálculo mecánico ajeno al objetivo | Internalizar en lectura/escritura validada | B:780 |
| cortex_ia_openspec_validate | Detectar estructura inválida | Llamada adicional tras escritura | Internalizar ejecución automática; diagnóstico opcional | B:792 |
| cortex_ia_change_archive | Cierre con aprobaciones vigentes | Operación excepcional de autoridad | Mantener planner | B:805 |
| cortex_ia_openspec_write | Escritura acotada de contratos | Necesaria donde planner no dispone de edit | Mantener planner | B:818 |
| cortex_ia_discovery_write | Guardar perfil acotado | Superficie especializada | Mantener discovery | B:841 |
| cortex_ia_doc_convert | Extraer documentos | Inventario universal; salida escrita necesita autoridad | Opcional según entrada; separar lectura/escritura | B:881 |
| cortex_ia_diagram_validate | Validar diagrama | Utilidad no necesaria en toda tarea | Opcional por habilidad | B:900 |
| cortex_ia_diagram_render | Generar diagrama | Artefacto escrito necesita autoridad | Opcional, con destino autorizado | B:916 |
| cortex_ia_board_create | Agrupar iniciativa | Paso separado al plan | Mantener o creación atómica con plan | B:932 |
| cortex_ia_board_list | Encontrar tableros | Solapa otras consultas | Fusionar consulta acotada | B:942 |
| cortex_ia_board_status | Vista del DAG | Solapa work_list/status | Fusionar vista de consulta | B:948 |
| cortex_ia_work_create | Materializar contrato ejecutable | Muchos campos, pero alcance útil | Mantener creación validada | B:954 |
| cortex_ia_work_review_refresh | Reabrir revisión por drift | Recuperación excepcional | Mantener solo orchestrator | B:998 |
| cortex_ia_work_list | Localizar tareas | Puede recuperar demasiado | Fusionar consulta con scope | B:1006 |
| cortex_ia_work_status | Estado/dependencias/autoridad | Respuesta grande; argumento role ignorado | Mantener vista canónica compacta | B:1016 |
| cortex_ia_work_approvals | Historial de revisión | Última aprobación ya está en status | Fusionar historial opcional | B:1050 |
| cortex_ia_work_fingerprint | Diagnosticar binding | Servicio ya impone comparación | Internalizar; diagnóstico disponible | B:1058 |
| cortex_ia_work_recover | Caducidad de autoridad | Alcance global puede afectar otras tareas | Mantener operación explícita y acotarla | B:1066 |
| cortex_ia_work_retry | Nuevo intento autorizado | Paso necesario contra fallback silencioso | Mantener orchestrator | B:1072 |
| cortex_ia_work_decompose | Sustituir tarea por DAG controlado | Sobrecoste fuera de cambios amplios | Mantener excepcional planner | B:1078 |
| cortex_ia_work_claim | Adquirir tarea y reservas atómicas | Expone mecánica al modelo | Mantener backend; acción semántica iniciar | B:1108 |
| cortex_ia_work_renew | Renovar claim | Depende de que modelo recuerde heartbeat | Internalizar en controlador vivo | B:1134 |
| cortex_ia_file_reserve | Añadir reserva de archivo | Claim ya reserva lote inicial | Internalizar, preservar lease/CAS | B:1145 |
| cortex_ia_work_lease_renew | Renovar reserva individual | N llamadas por heartbeat | Internalizar renovación conjunta | B:1171 |
| cortex_ia_work_release_all | Liberar todas las reservas | Puente hace N operaciones pese al batch CLI | Fusionar entrega/parada con servicio batch | B:1190 |
| cortex_ia_file_release | Liberar reserva individual | Contabilidad mecánica | Internalizar | B:1211 |
| cortex_ia_work_transition | Entregar o bloquear trabajo | Campos de evidencia y limpieza fragmentados | Mantener entrega semántica atómica | B:1255 |
| cortex_ia_work_approve | Aprobación independiente | No puede integrarse en autocompletado implement | Mantener reviewer | B:1288 |
| cortex_ia_delegate_start | Elegir modo y aceptar ejecución | Campos legacy rechazados; se carga en nativo | Mantener cuando política/capacidad permiten; esquema vigente | B:1314 |
| cortex_ia_delegation_status | Leer trabajo externo | Solapa wait/result | Fusionar consulta job | B:1529 |
| cortex_ia_delegation_wait | Esperar sin duplicar ejecución | Polling y formatos repetidos | Fusionar consulta con espera acotada | B:1535 |
| cortex_ia_delegation_result | Obtener receipt | Requiere consultar estado adicional | Fusionar receipt terminal en consulta | B:1613 |
| cortex_ia_delegation_cancel | Solicitar terminación | Solicitud no equivale a terminación confirmada | Mantener explícita | B:1650 |
| cortex_ia_delegation_recover | Marcar ejecución perdida | Mantenimiento del ciclo de vida | Mantener scoped orchestrator | B:1662 |
| cortex_ia_delegation_reconcile | Liberar cuarentena con prueba | Extraordinaria, no ruta general de fallback | Mantener solo orchestrator | B:1668 |
| cortex_ia_report_error | Reporte vinculado a incidente | LLM repite errores ya conocidos por runtime | Internalizar reporte saneado; anotación opcional | B:1680 |
| cortex_ia_snapshot_read | Leer texto exacto y verificar pin | Exporta hasta 8 MiB para una observación; retries | Mantener lectura verificada, optimizar acceso por ID | `internal/assets/plugins/cortex-snapshot.ts:8` |

Los otros plugins son hooks/adaptadores y no añaden herramientas Cortex-IA. Un hook que rechaza ejecución no reduce por sí mismo los schemas enviados al modelo: filtrado de inventario y autorización deben mantenerse como capas distintas.

## Próximo diseño, sin implementación en esta auditoría

El camino habitual debería pedir al modelo decidir objetivo, alcance, evidencia y resultado; el controlador debería ejecutar heartbeat, reservas, hashes, validación estructural y saneamiento. Una acción de entrega puede agrupar transición y liberación sin mezclar aprobación independiente. La consulta de tarea/job puede ofrecer vistas compactas y detalle bajo demanda. La recuperación extraordinaria sigue explícita: no se debe esconder tras un supuesto «continuar» que libere autoridad sin prueba.

Flujo nativo observado: planner crea board/tarea; implement consulta, reclama con paths, consulta política de delegación, edita con guard, renueva claim y cada lease, verifica y entrega; reviewer consulta, inspecciona y aprueba. Hoy no hay heartbeat automático del controlador nativo: session.idle registra/borra tracking y workAuthority vive en memoria (B:40, :99, :726, :729). La renovación conjunta existente para worker externo (`work.go:759`) no debe reutilizarse como endpoint sin autenticación. TTL sigue siendo necesario para autoridad abandonada.

La resolución del ejecutable se repite y el adaptador usa procesos síncronos sin timeout (B:307–398). Tres operaciones simuladas claim/transition/doc generaron diez subprocessos, incluidos cinco probes de versión: es un recuento del adaptador, no una medición de latencia real. Cachear resolución con invalidación, acotar duración y usar el release-all transaccional ya disponible reduce ese coste. La liberación best-effort tragada tras transición (B:1280) debe producir estado explícito de recursos pendientes.

Dos alternativas viables: **A, simplificación compatible**, conserva nombres y primitivas pero compacta consultas, automatiza mantenimiento autenticado y corrige evidencia/destinos. **B, interfaz semántica por rol**, ofrece consultar/planificar/iniciar/entregar/revisar y ejecución externa opcional; el controlador traduce a primitivas SQLite existentes. Recomendada B por etapas empezando por A. Evitar una megaherramienta action:any: menos nombres no garantiza menos privilegios. Las operaciones cancelar, recuperar y reconciliar conservan significados distintos. Reviewer/investigate tienen bash ampliamente permitido: la política del rol no es un sandbox del sistema operativo.

Orden propuesto: primero autoridad de destinos y evidencia de entrega; en paralelo corregir preparación/autenticación AGY y contratos de capacidades; después automatizar mecánica y reducir superficies. La evaluación de inventario aquí usa plugins reales con mocks y YAML, no una captura de request al proveedor. La compatibilidad de filtrado OpenCode 1.18.29 se comprobó anteriormente con funciones originales; no se presenta como E2E actual.

Este informe no modifica producto, configuración ni contratos archivados, y no resuelve todavía la compatibilidad actual de AGY. Las recomendaciones requieren diseño e implementación posteriores con pruebas de autoridad y comportamiento; no justifican retirar primitivas internas que hoy sostienen la seguridad.
