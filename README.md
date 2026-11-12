# Ardora - StockController

Sistema de gestão integrado para luderias, bares e cafés de jogos de tabuleiro, combinando controle de comandas, estoque, catálogo de acervo e gerenciamento de mesas em uma aplicação desktop nativa, rápida e offline-first.

---

## 🎲 Visão Geral

O **Ardora StockController** foi desenvolvido para simplificar a operação diária de espaços focados em jogos de tabuleiro. O sistema elimina atritos operacionais no check-in de clientes, controle de comandas, consumo de produtos do bar, ocupação de mesas e histórico de partidas disputadas.

A aplicação funciona de maneira totalmente local (offline-first), garantindo confiabilidade mesmo sem conexão com a internet.

---

## ✨ Principais Funcionalidades

### 🏠 Visão Geral (Home Dashboard)
- **Métricas em tempo real:** Contadores de clientes presentes, mesas ativas, jogos em andamento e faturamento do dia.
- **Acesso rápido:** Atalhos para iniciar novas comandas, alocar mesas e consultar status operacional de imediato.

### 👥 Comandas & Vendas (PDV)
- **Controle por Comanda:** Vinculação de comandas numeradas (0 a 999) por cliente.
- **Códigos Físicos Reutilizáveis:** Geração em lotes com QR Code e Code 128, impressão A4 e liberação automática após o fechamento.
- **Taxa de Entrada & Consumo:** Registro rápido de taxa de entrada (couvert/day use) e itens consumidos.
- **Fechamento de Conta:** Cálculo automático com suporte a descontos customizados.
- **Métodos de Pagamento:** Suporte a Dinheiro, Pix, Cartão de Crédito, Cartão de Débito e Outros.
- **Comprovante Não Fiscal:** Geração e armazenamento de recibos itemizados em texto para impressão ou consulta.

### 🎲 Catálogo de Jogos
- **Gestão de Acervo:** Cadastro completo com número de jogadores (mínimo e máximo), notas/regras e fotos de capa.
- **Filtros e Busca:** Localização ágil por nome com paginação e suporte a status ativo/inativo.

### 📦 Estoque & Produtos
- **Catálogo de Produtos:** Categorização (Bebidas, Salgados, Doces, Acessórios, etc.) e precificação em centavos para precisão monetária absoluta.
- **Alerta de Estoque Mínimo:** Sinalização visual quando itens atingem limite crítico de reposição.
- **Upload de Imagens:** Armazenamento local otimizado das fotos de cada produto.

### 🪑 Gestão de Mesas
- **Mapa de Mesas:** Monitoramento visual de mesas disponíveis e ocupadas.
- **Sessões de Partidas:** Registro do jogo em andamento — tanto jogos do catálogo próprio quanto jogos trazidos pelos clientes.
- **Histórico de Ocupação:** Registro de entrada e saída de clientes por mesa ao longo da estadia.

### ⚙️ Configurações & Backups
- **Tema Claro / Escuro:** Alternância de tema com persistência das preferências do usuário.
- **Backups CSV:** Exportação automática e sob demanda de relatórios e dados diários.

---

## 🧾 Operação com Códigos de Comanda

- Leitores USB de QR Code ou código de barras funcionam como teclado: mantenha o campo contextual focado e configure o leitor para finalizar cada leitura com `Enter`.
- O QR Code e o Code 128 impressos em uma etiqueta carregam exatamente o mesmo valor textual, como `001`; o sistema não diferencia a tecnologia usada na leitura.
- No check-in, a leitura apenas valida e preenche o código. A comanda só é atribuída ao cliente após a confirmação explícita em **Registrar entrada**.
- Em **Lançar produto**, selecione o produto e a quantidade antes de ler a comanda. Em **Fechar comanda**, a leitura apenas abre a prévia; o fechamento e o pagamento continuam explícitos.
- **Imprimir** ou reimprimir um lote existente nunca reserva novos números. Somente **Gerar códigos** avança a sequência.
- Nos catálogos, **Excluir** se torna **Arquivar** quando o item possui histórico. Cadastros arquivados permanecem em relatórios e podem ser reativados pela edição.

---

## 🏛️ Arquitetura do Projeto

O projeto é estruturado em Go seguindo separação limpa entre backend, frontend e modelos de domínio:

```
StockController/
├── internal/
│   ├── backend/
│   │   ├── database/       # Conexão SQLite (modernc) e migrations versionadas
│   │   ├── models/         # Entidades de domínio (Client, Product, Game, Visit, etc.)
│   │   ├── paths/          # Resolução segura de caminhos (%LOCALAPPDATA%\Ardora)
│   │   └── services/       # Regras de negócio (Games, Products, Sales, Tables, Visits, etc.)
│   └── frontend/
│       ├── components/     # Componentes visuais reutilizáveis (botões, cards, diálogos)
│       ├── icons/          # Ícones vetoriais embutidos
│       ├── style/          # Tema, paleta de cores e tipografia
│       ├── tabs/           # Telas da aplicação (Home, Games, Stock, Sales, Tables, Settings)
│       └── state.go        # Estado global e navegação entre abas
├── main.go                 # Inicialização da janela desktop, runtime e splash
├── main_test.go            # Testes de renderização visual
└── theme.go                # Customização de estilos da aplicação
```

---

## 💾 Armazenamento de Dados

Todos os dados locais da aplicação ficam isolados no diretório de dados do usuário:

- **Windows:** `%LOCALAPPDATA%\Ardora\`
  - `ardora.db`: Banco de dados SQLite transacional com migrations automáticas.
  - `images/`: Imagens de produtos e jogos de tabuleiro cadastradas.
  - `receipts/`: Arquivos de comprovantes não fiscais emitidos.
  - `backups/`: Exportações periódicas em CSV.

> [!NOTE]
> Todos os valores financeiros são persistidos em centavos (`INTEGER`) para evitar imprecisões e erros de arredondamento de ponto flutuante.

---

## 🚀 Como Executar

### Pré-requisitos
- [Go](https://go.dev/dl/) 1.22 ou superior instalado.

### Instalação de Dependências
```bash
go mod download
```

### Executar em Desenvolvimento
```bash
go run .
```

### Executar Testes
```bash
go test ./...
```

### Build do Executável (Windows)
```bash
# Compilar binário padrão
go build -o ardora.exe .

# Compilar para produção sem exibir console/terminal no fundo
go build -ldflags "-H windowsgui" -o ardora.exe .
```

---

## 📄 Licença

Este projeto está licenciado sob a Licença MIT – consulte o arquivo [LICENSE.txt](LICENSE.txt) para obter detalhes.
