# ESTELA

Arena de estelas de neón en pixel art, para la terminal. Hasta 6 jugadores en la misma red Wi-Fi: uno crea la sala, comparte un código de 7 letras y el resto se une. Los lugares vacíos los ocupan bots.

## Jugar

```sh
go build -o estela ./cmd/estela
./estela                    # menú
./estela --solo             # vos contra 3 bots
./estela --host             # crear sala y mostrar el código
./estela --join NG8-05A0    # unirse con un código
./estela --demo             # todos son bots, para mirar
./estela --name ANA         # elegir tu nombre (máx. 10 letras)
```

La terminal tiene que medir al menos **100×32** y conviene que tenga color verdadero (iTerm2, WezTerm, Kitty, Ghostty, Windows Terminal). En terminales de 256 colores también anda.

## Controles

| Tecla | Acción |
| --- | --- |
| Flechas o WASD | Girar |
| Espacio | Impulso: 1,5 s al doble de velocidad. Gasta energía, que se recarga sola |
| J, X o Enter | Salto: 3 casillas sin dejar estela y pasando por encima de otras. Se recarga en 4 s |
| Esc o Q | Salir de la partida |

En la sala, el anfitrión usa **+/-** para sumar o sacar bots y **Enter** para empezar.

## Reglas

- Chocar contra una pared o una estela te elimina. Dos cabezas que chocan de frente quedan las dos afuera.
- Las **chispas** que aparecen en la arena te llenan el impulso y te recargan el salto.
- A los 30 segundos empieza el **colapso**: el borde se cierra un anillo cada 2 segundos.
- Gana la ronda el último que queda en pie. Gana la partida el primero en llegar a 5 rondas.

## Jugar en red

El código de la sala guarda la IP local y el puerto del anfitrión, así que unirse no depende de que la red deje descubrir dispositivos. El menú además lista las salas abiertas que detecta en la red, si la red lo permite.

Si alguien no se puede unir:

- **macOS** puede pedir permiso de **Red local** para la terminal la primera vez (Ajustes del Sistema → Privacidad y seguridad → Red local). Hay que aceptarlo en las dos máquinas.
- Las redes de invitados o con "aislamiento de clientes" no dejan que los dispositivos se hablen entre sí. Ahí no hay código que valga: usen otra red o compartan internet desde un teléfono.
- El anfitrión escucha en TCP 47470–47477 y anuncia la sala por UDP 47570.

## Compilar para tus amigos

```sh
GOOS=darwin  GOARCH=arm64 go build -o dist/estela-mac       ./cmd/estela
GOOS=linux   GOARCH=amd64 go build -o dist/estela-linux     ./cmd/estela
GOOS=windows GOARCH=amd64 go build -o dist/estela.exe       ./cmd/estela
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
