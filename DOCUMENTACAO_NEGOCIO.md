# Documentação de Negócio

## 1. Objetivo do produto

A plataforma colaborativa de descoberta organiza um catálogo de cervejas e permite que uma comunidade registre avaliações, comentários e interações sobre cada produto.

O objetivo do negócio é oferecer uma experiência participativa para:

- consultar cervejas e suas características;
- descobrir produtos por busca, estilo e filtros;
- registrar avaliações e comentários;
- acompanhar a atividade recente da comunidade;
- administrar conteúdo inadequado;
- proteger os dados e as permissões dos participantes;
- incentivar o retorno e a participação contínua da comunidade.

## 2. Participantes

### Visitante

Pessoa que consulta informações públicas da plataforma sem criar uma conta.

Pode consultar o catálogo, realizar buscas, visualizar detalhes das cervejas e acessar informações gerais disponíveis ao público.

### Usuário

Pessoa cadastrada que participa da comunidade.

Pode criar sua conta, entrar na plataforma, consultar seu perfil, propor cervejas e interagir com comentários conforme as regras de participação.

### Administrador

Pessoa autorizada a cuidar da operação e da qualidade da comunidade.

Pode administrar conteúdos e recursos que exigem uma visão global, inclusive tratar denúncias, avaliar o catálogo e decidir sobre exclusões.

### Serviço de moderação

Mecanismo que verifica se comentários e conteúdos enviados respeitam as diretrizes da comunidade.

Quando um conteúdo é considerado inadequado, ele não deve ser aceito como uma interação normal.

## 3. Catálogo de cervejas

Cada cerveja pode conter informações como:

- nome;
- estilo;
- descrição;
- teor alcoólico;
- sabor;
- aroma;
- cor;
- corpo;
- carbonatação;
- finalização;
- local de compra;
- referência de localização;
- comentários e avaliações;
- histórico de alterações.

O catálogo é colaborativo. Divergências, classificações diferentes e debates fazem parte da experiência de descoberta.

A plataforma deve reduzir duplicidades, mas não precisa eliminar toda divergência. O objetivo é oferecer contexto, participação e uma avaliação administrativa para situações que afetem a organização do catálogo.

## 4. Cadastro de cervejas

Um usuário autenticado pode propor uma nova cerveja.

Ao cadastrar uma cerveja:

1. a plataforma identifica o usuário responsável pela contribuição;
2. registra a data de criação;
3. verifica se existe uma cerveja semelhante;
4. informa sugestões quando encontra possíveis duplicidades;
5. grava a nova cerveja quando não há conflito relevante.

O usuário responsável pode propor correções nas informações. A exclusão de cervejas é uma decisão exclusiva de administradores, pois o registro pode conter contribuições de toda a comunidade.

## 5. Avaliações e comentários

A comunidade pode comentar e avaliar cervejas.

Cada comentário pode conter texto, nota e identificação de seu autor.

As notas válidas são de 1 a 5. A média e a quantidade de avaliações são calculadas a partir dos comentários válidos.

A confiabilidade absoluta da nota não é uma promessa do produto. Opiniões diferentes e debates fazem parte do atrativo da comunidade. O valor está em permitir descoberta, expressão de preferências e troca de experiências.

Antes de ser publicado, o texto pode passar por uma verificação de conteúdo. Comentários que violam as diretrizes da comunidade devem ser recusados.

Comentários e demais registros da comunidade não devem ser excluídos por usuários. A exclusão de conteúdo é uma decisão administrativa, aplicada quando necessária para proteger a comunidade, corrigir abusos ou manter a organização do catálogo.

## 6. Interações da comunidade

Usuários autenticados podem indicar que gostaram de um comentário.

A plataforma identifica a pessoa que realizou a interação para evitar que o mesmo participante registre vários likes no mesmo comentário.

A interação também pode ser associada ao dispositivo quando não houver identificação de usuário, conforme a experiência disponível no produto.

## 7. Regras de acesso

As permissões seguem estas regras de negócio:

- informações públicas podem ser consultadas sem autenticação;
- ações de participação exigem uma conta autenticada;
- um usuário pode consultar e administrar o próprio perfil;
- um usuário não pode acessar o perfil privado ou administrar os dados de outro usuário;
- o responsável por uma cerveja pode propor correções nas informações;
- somente um administrador pode excluir cervejas, comentários ou outros registros;
- um administrador pode avaliar e decidir sobre conteúdos de terceiros;
- recursos administrativos são exclusivos para administradores;
- o papel de administrador não pode ser escolhido livremente durante o cadastro;
- a renovação da sessão mantém o papel atual do usuário.

## 8. Contas e acesso à plataforma

O cadastro cria uma conta comum. Informar que deseja ser administrador não concede esse privilégio.

O acesso pode ocorrer por credenciais próprias ou por um provedor de identidade integrado.

A plataforma utiliza sessões com duração limitada. Quando uma sessão expira, o usuário pode solicitar sua renovação usando o recurso de renovação disponível.

Tokens de renovação são individuais e podem ser invalidados. O uso de um token já invalidado deve ser tratado como uma tentativa de reutilização indevida e pode invalidar as demais sessões do usuário.

As regras de identidade e de acesso social podem evoluir conforme as necessidades da comunidade e as diretrizes de governança forem amadurecendo.

## 9. Denúncias e moderação

Usuários podem denunciar cervejas ou conteúdos que considerem inadequados.

As denúncias são tratadas como solicitações de moderação e ficam disponíveis para administradores.

O administrador pode:

- consultar denúncias;
- analisar o conteúdo denunciado;
- resolver uma denúncia;
- avaliar registros controversos ou incompletos do catálogo;
- decidir pela exclusão de uma cerveja ou conteúdo quando aplicável;
- acompanhar solicitações de exclusão.

O objetivo é preservar a qualidade do catálogo e a segurança da comunidade sem dar a qualquer usuário poder de exclusão.

A moderação deve evoluir com o produto, acompanhando os tipos de abuso, os debates recorrentes e as necessidades observadas na comunidade.

## 10. Estatísticas e acompanhamento

A plataforma oferece informações de acompanhamento para diferentes públicos.

Usuários podem consultar suas próprias estatísticas, como participação e contribuições.

Administradores podem consultar indicadores gerais da operação, como volume do catálogo, participação da comunidade e distribuição das atividades.

Essas informações servem principalmente para incentivar o uso: mostrar participação, evolução e contribuição ajuda o usuário a perceber valor em continuar descobrindo e colaborando. Administradores também podem usar os indicadores para acompanhar a evolução do produto.

## 11. Eventos e histórico interno

Alterações relevantes realizadas no catálogo geram registros internos de atividade.

Esses registros existem para:

- medir comportamento e participação da comunidade;
- acompanhar indicadores do negócio;
- identificar tendências de uso;
- apoiar decisões administrativas;
- investigar ocorrências autorizadas.

Os eventos são extremamente restritos e não fazem parte da experiência pública dos usuários. Seu acesso deve ser limitado às pessoas e funções autorizadas para métricas, gestão e investigação do negócio.

A indisponibilidade de um mecanismo auxiliar de registro não deve apagar a operação que já foi confirmada.

## 12. Disponibilidade e continuidade

A plataforma diferencia serviços essenciais de serviços de apoio.

Quando a base de dados não estiver disponível:

- consultas e ações que dependem dela devem informar indisponibilidade;
- verificações de funcionamento devem continuar indicando se o serviço está vivo;
- a plataforma não deve confirmar uma operação que não conseguiu registrar;
- a indisponibilidade de um recurso auxiliar não deve desfazer uma operação já confirmada.

O catálogo e os registros de negócio são a fonte principal das informações. Recursos auxiliares existem para melhorar velocidade, acompanhamento e experiência, mas não substituem os registros oficiais.

## 13. Proteção dos dados

A plataforma deve:

- limitar o acesso conforme o papel e a relação do usuário com o recurso;
- evitar que mensagens de erro revelem informações internas ou pessoais;
- proteger credenciais e sessões;
- limitar entradas excessivamente grandes;
- tratar conteúdos enviados antes de exibi-los a outras pessoas;
- registrar informações suficientes para investigação de falhas sem expor dados desnecessários.

## 14. Operação do negócio

A operação deve preservar os dados existentes por padrão.

A recriação completa dos dados é uma ação excepcional, adequada apenas para ambientes descartáveis, desenvolvimento ou testes controlados. Em uma operação real, a atualização deve preservar o catálogo, as contas, as avaliações, as denúncias e os registros internos.

## 15. Fora do escopo atual

O envio de arquivos de mídia não faz parte do fluxo público atual. A plataforma pode armazenar referências de mídias já existentes, mas o processo de receber, validar e armazenar novos arquivos depende de uma decisão futura de produto e operação.

## 16. Resultados esperados

A plataforma deve proporcionar:

- descoberta colaborativa de cervejas;
- participação segura da comunidade;
- espaço para opiniões diferentes e debates;
- separação clara entre usuário comum e administrador;
- avaliação administrativa do catálogo;
- métricas internas protegidas para orientar o negócio;
- tratamento responsável de conteúdo inadequado;
- preservação dos dados em atualizações e indisponibilidades parciais;
- incentivos para participação e retorno à plataforma.

## 17. Evolução prevista

As próximas evoluções devem seguir as diretrizes já definidas:

- ampliar os papéis administrativos conforme o volume e a complexidade da comunidade aumentarem;
- aperfeiçoar a moderação a partir dos casos reais observados;
- evoluir a gestão de identidade e contas conforme surgirem novos meios de acesso;
- considerar notificações e outros incentivos quando houver uma necessidade clara de engajamento;
- manter os eventos de negócio restritos e voltados a métricas internas;
- adiar novos recursos que não fortaleçam a descoberta colaborativa.
