# lazymanager

TUI para gerenciar pacotes, no estilo do lazygit/lazydocker. Reúne os
gerenciadores do sistema (Linux, *BSD, Homebrew no macOS) e, quando executado
dentro de um repositório, também o gerenciador de pacotes do projeto.

## Instalação

```sh
go install .
```

Ou, a partir do código-fonte:

```sh
go build -o lazymanager .
```

## Uso

```sh
lazymanager            # inspeciona o diretório atual
lazymanager -C ~/proj  # inspeciona outro diretório
lazymanager --version
```

## Teclas

| Tecla | Ação |
| --- | --- |
| `tab` / `shift+tab` | Alterna o painel focado |
| `1` `2` `3` `4` / `h` `l` / `←` `→` | Foca gerenciadores / pacotes / atualizações / detalhes |
| `j` `k` / `↑` `↓` | Move a seleção |
| `pgup` / `pgdown` | Move uma página nas listas / rola os detalhes |
| `g` / `G` | Início / fim |
| mouse | Clique foca e seleciona; a roda move a lista ou rola os detalhes |
| `/` | Filtra pacotes (case-insensitive; `esc` limpa) |
| `s` | Busca no repositório remoto |
| `i` | Instala um pacote |
| `d` | Remove o pacote selecionado (confirma) |
| `u` | Atualiza o pacote ou atualização selecionada (confirma) |
| `U` | Atualiza todos (confirma) |
| `A` | Atualiza tudo: todos os gerenciadores do sistema com atualizações pendentes |
| `r` | Recarrega pacotes e atualizações do gerenciador atual |
| `enter` | Carrega os detalhes do pacote ou atualização |
| `?` | Ajuda |
| `q` / `ctrl+c` | Sai |

Comandos que alteram o sistema rodam dentro da TUI: a saída aparece ao vivo no
painel **Details**. Quando o `sudo` precisa de senha, um modal mascarado a pede;
a senha só é enviada ao `sudo -S -v` via stdin (nunca como argumento) e, depois,
os comandos rodam com `sudo -n`. Isso depende do cache de credenciais do sudo
(`timestamp_timeout` maior que zero, que é o padrão).

## Painel de atualizações

Os quatro painéis ficam assim: **Managers**, **Packages** e **Updates** na
coluna da esquerda, e **Details** na coluna da direita. As atualizações são
carregadas em segundo plano ao trocar de gerenciador e ficam em cache por nome,
então voltar a um gerenciador já visitado é instantâneo.

O título mostra `Updates (N)`. Quando o gerenciador não suporta listar
atualizações, o painel mostra `updates not supported for <nome>`. Quando não há
nada pendente, aparece um pequeno aviso verde de "all good". A partir de 30
atualizações (`manyUpdates`) o título ganha `⚠` em amarelo e a barra de status
mostra um alerta como `dnf: 120 updates pending — press U to upgrade all`.

No painel de atualizações, `enter` mostra as informações do pacote em Details,
`u` atualiza o item selecionado e `U` atualiza tudo, sempre com confirmação.
Depois de qualquer comando que altere o sistema, pacotes e atualizações são
recarregados.

Suportam listagem de atualizações: `apt`, `dnf`, `pacman`, `brew`, `flatpak`,
`go`, `npm`, `pip`, `uv` e `poetry`. Os demais gerenciadores continuam
disponíveis para as outras ações, mas mostram que atualizações não são
suportadas.

## Gerenciadores suportados

Sistema: `apt`, `dnf`, `pacman`, `zypper`, `apk`, `xbps`, `pkg` (FreeBSD),
`brew`, `flatpak`.

Projeto: `go` (`go.mod`), `npm` (`package.json`), `pnpm` (`pnpm-lock.yaml`),
`yarn` (`yarn.lock`), `bun` (`bun.lockb`/`bun.lock`), `cargo` (`Cargo.toml`),
`uv` (`uv.lock`), `poetry` (`poetry.lock`), `pip` (`requirements.txt`),
`bundle` (`Gemfile`), `composer` (`composer.json`).

Só aparecem os gerenciadores cujo binário está no `PATH`; os de projeto também
precisam do arquivo de detecção no diretório inspecionado. Quando existe um
lockfile de `pnpm`, `yarn` ou `bun`, o `npm` não é oferecido.

## Estrutura

```
.
├── main.go
├── internal
│   ├── manager
│   │   ├── manager.go
│   │   ├── table.go
│   │   ├── parse.go
│   │   └── manager_test.go
│   └── ui
│       ├── model.go
│       ├── update.go
│       ├── mouse.go
│       ├── view.go
│       ├── run.go
│       └── ui_test.go
├── go.mod
└── go.sum
```
