# Códigos reutilizáveis, CRUD operacional e correção das comandas

## Intenção

Evoluir o atendimento da Ardora para que cada visita receba temporariamente um código físico reutilizável. O mesmo valor deverá ser legível por QR Code ou código de barras e será usado de forma contextual para abrir uma comanda, lançar produtos e localizar a conta no fechamento. A mudança também corrige o overflow do estado vazio das comandas e adiciona edição/exclusão segura para mesas, produtos e jogos.

## Objetivos

- Cadastrar códigos reutilizáveis em lotes sequenciais sem duplicações.
- Associar exatamente um código disponível a uma visita aberta e liberá-lo no checkout.
- Aceitar a mesma leitura textual por QR Code, código de barras, digitação manual ou futuros adaptadores de câmera.
- Gerar etiquetas com número visível, QR Code e Code 128 contendo o mesmo valor.
- Reimprimir qualquer lote sem avançar a sequência nem criar novos códigos.
- Expor edição e exclusão em Mesas, Estoque e Jogos com preservação do histórico.
- Eliminar a sobreposição entre o estado vazio de comandas e o painel de recibos.

## Não objetivos

- Controlar câmera, leitor serial, NFC ou equipamento específico nesta versão.
- Cadastrar códigos de barras próprios dos produtos; o leitor contextual identifica a comanda, enquanto o produto continua sendo selecionado na interface.
- Permitir que um código identifique duas visitas abertas ao mesmo tempo.
- Recuperar números excluídos, preencher lacunas ou renumerar códigos já emitidos.
- Criar designer de etiquetas, escolher dimensões arbitrárias ou integrar impressoras térmicas.
- Remover ou reescrever registros históricos para permitir exclusões.

## Antiobjetivos

- Não depender de rede para ler, gerar ou imprimir códigos.
- Não aceitar silenciosamente uma leitura em uma tarefa diferente da que está visível.
- Não realizar exclusão física quando isso quebraria vendas, visitas, sessões de jogo ou alocações antigas.
- Não manter duas fontes de verdade independentes para a disponibilidade de um código.

## Restrições atuais

- Aplicação desktop local em Go/MyGo com SQLite e uma única conexão de escrita.
- O modelo atual usa `clients.active_card` e `visits.card_number`, limitados a `0–999`.
- Migrações são somente aditivas; migrações publicadas não são alteradas.
- O fluxo de impressão existente já gera HTML local e delega ao sistema operacional/navegador.
- Produtos, jogos e mesas possuem referências históricas que impedem exclusão física indiscriminada.

## Abordagens consideradas

### Cadastro explícito de códigos reutilizáveis — escolhido

Criar entidades para lotes e códigos. Disponibilidade é derivada da existência de uma visita aberta vinculada ao código. Essa abordagem permite validar leituras, reimprimir lotes, arquivar códigos perdidos e impedir duplicações com restrições do banco.

### Apenas continuar o cartão numérico atual

Guardar somente o último número em configuração seria menor, mas não permitiria consultar lotes, reimprimir com segurança, arquivar um código individual ou explicar por que um número está indisponível.

### Credenciais genéricas e múltiplas tecnologias

Uma camada genérica para QR, barras, NFC e pulseiras seria flexível, porém exigiria migração e abstrações maiores do que o problema atual pede.

## Modelo de dados

### `access_code_batches`

- `id`
- `first_number` e `last_number`
- `quantity`
- `created_at`
- `last_printed_at`, opcional

Cada geração cria um lote imutável. A quantidade solicitada determina um intervalo contínuo. O último número emitido é preservado transacionalmente; gerar um lote nunca reutiliza lacunas.

### `access_codes`

- `id`
- `batch_id`
- `number`, inteiro único
- `value`, texto único já formatado para leitura, como `001`
- `active`
- `created_at` e `archived_at`

O estado apresentado é derivado:

- **Disponível:** ativo e sem visita aberta.
- **Em uso:** vinculado a uma visita aberta.
- **Arquivado:** `active = false` e sem possibilidade de novo check-in.

### Visitas

Adicionar `visits.access_code_id`. Uma restrição parcial única impede que um código esteja em mais de uma visita aberta. `card_number` continua como snapshot e compatibilidade durante a migração, mas as novas regras consultam `access_code_id`.

Na migração, os números distintos já encontrados nas visitas formam um lote especial **Legado** e são relacionados aos códigos correspondentes. Códigos legados vinculados somente a visitas encerradas ficam disponíveis imediatamente; códigos vinculados a visitas abertas permanecem em uso até o checkout. O valor `0`, se existir, torna-se `000`; novas gerações começam em `001` ou após o maior número legado. A primeira versão permanece no intervalo `000–999` para respeitar o modelo atual e apresenta erro de capacidade ao atingir o limite.

## Geração e impressão

Uma nova área **Códigos** na navegação mostra:

- quantidade total, disponíveis, em uso e arquivados;
- próximo número a ser emitido;
- formulário para informar a quantidade do próximo lote;
- histórico de lotes com intervalo, quantidade, data e ação **Imprimir**;
- listagem pesquisável dos códigos e seus estados.

A geração ocorre em uma única transação: valida o limite, reserva o intervalo, cria o lote e insere todos os códigos. Falha parcial reverte tudo.

O botão **Imprimir** não altera a sequência. Ele recria um HTML local com CSS `@page size: A4`, distribui as etiquetas em múltiplas páginas e abre a impressão do sistema. Cada etiqueta contém:

- `ARDORA`;
- número grande e legível;
- QR Code;
- código de barras Code 128;
- o mesmo payload textual zero-padded em ambos, por exemplo `001`.

Imprimir novamente o mesmo lote produz os mesmos códigos. O usuário pode usar a impressão do sistema para papel ou PDF.

## Leitura contextual

Leitores USB normalmente operam como teclado. Cada tarefa oferece um campo de leitura focável que aceita caracteres e processa o valor ao receber `Enter`. Digitação manual permanece disponível para desenvolvimento e contingência.

Um normalizador remove espaços e quebras, valida o formato numérico e converte para o valor canônico zero-padded. Em seguida, o serviço consulta `access_codes`; não existe distinção entre o valor vindo de QR ou barras.

### Check-in

O campo atual **Cartão (0–999)** passa a ser **Código da comanda**. O atendente pode digitar, selecionar um código disponível ou escanear. O serviço rejeita código inexistente, arquivado ou já em uso. Ao abrir a visita, grava `access_code_id` e mantém o snapshot numérico.

A leitura, por si só, apenas normaliza e valida o valor, preenche o campo e mostra claramente o número da comanda localizada. Nenhuma associação ou cobrança é persistida nesse momento. O botão **Registrar entrada** permanece como confirmação explícita; somente ele cria a visita, registra a entrada e atribui o código em uma única transação. Se os dados do cliente ou a persistência falharem, o código continua disponível.

### Lançar produto

O atendente seleciona produto e quantidade, depois escaneia o código da comanda. Uma leitura válida localiza a visita aberta e lança o produto. Enquanto o lançamento está em andamento, novas leituras são ignoradas para impedir duplicação concorrente. Cada `Enter` concluído representa uma intenção de lançamento; leituras repetidas deliberadas geram novos lançamentos.

### Fechar comanda

Na área de comandas, uma ação **Localizar por código** recebe a leitura. Uma leitura válida abre a prévia da visita correspondente com entrada, consumo e descontos, mas não confirma o pagamento automaticamente. Após o checkout, o código volta a ficar disponível.

Erros aparecem junto à tarefa: código desconhecido, arquivado, disponível sem visita, já em uso, ausência de produto selecionado ou falha de persistência.

Um adaptador de câmera futuro só precisará entregar o mesmo texto ao normalizador e aos serviços atuais.

## Edição e exclusão segura

Mesas, produtos e jogos recebem ações **Editar** e **Excluir** nos seus cartões. A edição reutiliza o formulário atual preenchido e chama os métodos `Update` já existentes, com validações equivalentes à criação.

Antes da confirmação, o backend calcula o modo de remoção e a interface explica a consequência:

- **Nunca utilizado:** exclusão física definitiva.
- **Com histórico:** arquivamento por `active = false`.

O serviço revalida dentro da transação para evitar que o estado mude entre a confirmação e a execução.

### Produtos

Referências em `history` exigem arquivamento. Produtos arquivados deixam de aparecer no lançamento, permanecem em relatórios e podem ser reativados pela edição.

### Jogos

Referências em `history`, `table_game_sessions` ou `tables.game_id` exigem arquivamento. Jogos arquivados não podem iniciar novas sessões, mas continuam nomeados no histórico.

Renomear produto, jogo ou mesa corrige o nome do cadastro e também as consultas históricas que hoje leem esse nome por relacionamento, incluindo o CSV diário e as sessões de jogo. Não serão adicionados snapshots retroativos para esses nomes. Campos que já são snapshots do evento, como a descrição financeira gravada numa linha de produto, permanecem imutáveis em recibos antigos.

### Mesas

Referências em `tables_clients`, `table_game_sessions` ou `history` exigem arquivamento. Uma mesa ocupada ou com jogo ativo não pode ser arquivada até que clientes e sessão sejam encerrados. Mesas arquivadas deixam a visão operacional e podem ser reativadas.

As confirmações usam linguagem distinta: **Excluir definitivamente** ou **Arquivar**, nunca uma ação ambígua. Limpeza de imagens órfãs fica fora desta mudança.

## Correção do overflow

### Causa

`components.Panel` aplica `Grow(1)` incondicionalmente. Isso funciona quando o painel é filho de uma linha e precisa dividir a largura, mas em uma coluna vertical também permite que o painel seja encolhido para repartir a altura. O estado vazio interno exige `MinHeight(180)` e acaba desenhado além do painel encolhido, invadindo **Recibos recentes**.

### Correção estrutural

O painel padrão passa a dimensionar sua altura pelo conteúdo. Painéis usados lado a lado aplicam crescimento explicitamente no ponto de uso para dividir a largura. Assim, o componente não mistura expansão horizontal e vertical sob a mesma regra.

Um teste de regressão renderiza Vendas com zero comandas abertas e pelo menos um recibo recente nos tamanhos suportados, verificando a ordem e a separação vertical dos estados. As prévias claro/escuro em `1024×768` e `800×600` continuam sendo geradas.

## Tratamento de erros e consistência

- Geração de lote, associação ao check-in, lançamento e checkout são transacionais.
- Restrições únicas do SQLite são a última defesa contra números duplicados e uso simultâneo.
- Falha de impressão não desfaz lote nem checkout; o botão pode ser acionado novamente.
- Um código arquivado não pode estar em uso; arquivamento é bloqueado enquanto houver visita aberta.
- Exclusões físicas retornam erro amigável se uma referência aparecer durante a confirmação.
- Listagens são atualizadas após geração, check-in, lançamento, checkout, edição, exclusão ou arquivamento.

## Migração e compatibilidade

1. Criar tabelas de lotes/códigos e índices.
2. Adicionar `visits.access_code_id`.
3. Criar lote legado e códigos para números históricos distintos.
4. Relacionar visitas antigas por `card_number`.
5. Passar serviços e telas a usar o código explícito como fonte de verdade.
6. Manter os campos numéricos existentes como snapshots/compatibilidade nesta versão.

O banco deve ser copiado nos testes antes e depois da migração. Bancos novos e bancos populados precisam produzir o mesmo comportamento operacional.

## Validação e critérios de aceite

- Um lote parcial continua exatamente após o maior número já emitido.
- Duas gerações concorrentes não criam duplicações.
- Reimpressão não altera o lote nem o próximo número.
- QR e Code 128 da mesma etiqueta decodificam para o mesmo valor.
- Um código não abre duas visitas simultâneas e é liberado após checkout.
- Produto selecionado + leitura válida cria exatamente um lançamento.
- Leitura no fechamento abre a visita correta sem fechá-la automaticamente.
- Editar preserva identificadores e histórico.
- Excluir item nunca usado remove; excluir item referenciado arquiva.
- Itens arquivados deixam fluxos ativos e continuam legíveis no histórico.
- O estado vazio de comandas não ultrapassa seu painel nem encobre recibos.
- Testes unitários, migração, serviços, renderização, `go vet`, build e validação OpenSpec passam.

## Decisões deixadas ao implementador

- Biblioteca pure-Go usada para gerar QR Code e Code 128, desde que funcione offline em tempo de execução e seja coberta por teste de decodificação ou payload.
- Quantidade exata de etiquetas por folha, definida a partir de margens A4 legíveis e validada por renderização.
- Organização interna dos serviços e view models, preservando as fronteiras e comportamentos descritos acima.
