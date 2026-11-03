# VIBETRON

Arena de estelas de neón para la terminal. Hasta 6 jugadores en la misma red Wi-Fi: uno crea la sala, comparte un código de 7 letras y el resto se une. Los lugares vacíos los ocupan bots.

## Jugar

```sh
go build -o vibetron ./cmd/vibetron
./vibetron                    # menú
./vibetron --solo             # vos contra 3 bots
./vibetron --host             # crear sala y mostrar el código
./vibetron --join NG8-05A0    # unirse con un código
./vibetron --demo             # todos son bots, para mirar
./vibetron --name ANA         # elegir tu nombre (máx. 10 letras)
```

Los menús piden una terminal de al menos **100×32**. Durante la partida alcanza con el tamaño de la arena que eligió el anfitrión (de 60×21 a 140×44). Conviene color verdadero (iTerm2, WezTerm, Kitty, Ghostty, Windows Terminal); en terminales de 256 colores también anda.

## Controles

| Tecla | Acción |
| --- | --- |
| Flechas o WASD | Girar |
| Espacio | Impulso: 1,5 s al doble de velocidad. Gasta energía, que se recarga sola |
| J, X o Enter | Salto: 3 casillas sin dejar estela y pasando por encima de otras. Se recarga en 4 s |
| Esc o Q | Salir de la partida |

En la sala, el anfitrión elige:

| Tecla | Qué cambia |
| --- | --- |
| T | Tamaño de la arena: Chica 60×34, Mediana 70×40, Grande 100×56, Enorme 140×80 |
| V | Velocidad: Tranqui (10 pasos/s), Normal (15), Rápida (20), Furiosa (25) |
| + / - | Sumar o sacar bots |
| Enter | Empezar |

Todos ven en la sala qué arena se eligió y si su terminal alcanza.

## Reglas

- Chocar contra una pared o una estela te elimina. Dos cabezas que chocan de frente quedan las dos afuera.
- La estela mide **70 casillas**: la cola se apaga y se borra mientras avanzás.
- Las **chispas** que aparecen en la arena alargan tu estela 25 casillas, te llenan el impulso y te recargan el salto.
- Al morir, tu estela se recoge desde la cola y libera espacio.
- Al minuto empieza la **muerte súbita**: las estelas dejan de achicarse hasta que quede uno.
- Gana la ronda el último que queda en pie. Gana la partida el primero en llegar a 5 rondas.

## Jugar en red

El código de la sala guarda la IP local y el puerto del anfitrión, así que unirse no depende de que la red deje descubrir dispositivos. El menú además lista las salas abiertas que detecta en la red, si la red lo permite.

Si alguien no se puede unir:

- **macOS** puede pedir permiso de **Red local** para la terminal la primera vez (Ajustes del Sistema → Privacidad y seguridad → Red local). Hay que aceptarlo en las dos máquinas.
- Las redes de invitados o con "aislamiento de clientes" no dejan que los dispositivos se hablen entre sí. Ahí no hay código que valga: usen otra red o compartan internet desde un teléfono.
- El anfitrión escucha en TCP 47470–47477 y anuncia la sala por UDP 47570.

## Compilar para tus amigos

```sh
GOOS=darwin  GOARCH=arm64 go build -o dist/vibetron-mac       ./cmd/vibetron
GOOS=linux   GOARCH=amd64 go build -o dist/vibetron-linux     ./cmd/vibetron
GOOS=windows GOARCH=amd64 go build -o dist/vibetron.exe       ./cmd/vibetron
```

## Cómo está hecho

- `internal/sim`: el juego en sí. Solo usa enteros y un generador pseudoaleatorio con semilla, así que dos máquinas que aplican las mismas entradas llegan exactamente al mismo estado.
- `internal/net`: lockstep sobre TCP. El anfitrión ordena las entradas de todos en ticks de 50 ms y cada máquina simula por su cuenta. Cada segundo se compara un hash del estado para detectar desincronizaciones.
- `internal/render`: framebuffer RGB donde cada celda de la terminal son dos píxeles (`▀`), con brillo, partículas, sacudidas y viñeta.
- `internal/bot`: bots con relleno por inundación y personalidad propia.
- `internal/ui`: los menús y la partida, como una máquina de estados de escenas.

```sh
go test ./...
go test -bench . -benchmem ./internal/render ./internal/bot
```
