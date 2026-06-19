# Teste para candidatos à vaga de SRE

Candidatos para vaga de SRE na Foxbit devem responder este teste como parte do processo seletivo. 

A solução do teste deve estar disponível em um repositório online (Github, Bitbucket, etc) e o link deve ser encaminhado por **mensagem ou e-mail**. Pode ser um repositório público ou um repositório privado com acesso para o avaliador.

## Desafio

Este teste tem como objetivo avaliar as capacidades do candidato em entregar uma aplicação web simples aplicando boas práticas de desenvolvimento e deploy. O desafio é dividido em 2 partes:

- Aplicação web
- Deploy da aplicação web em um cluster de Kubernetes

### Aplicação web

A aplicação web consiste de uma API que executa as quatro operações básicas da matemática:
- divisão
- multiplicação
- subtração
- adição.

Devem existir 4 endpoints, 1 para cada operação:

#### Subtração
`GET /api/sub?term_one=<term_one>&term_two=<term_two>`

#### Multiplicação
`GET /api/mul?term_one=<term_one>&term_two=<term_two>`

#### Divisão
`GET /api/div?term_one=<term_one>&term_two=<term_two>`

#### Adição
`GET /api/sum?term_one=<term_one>&term_two=<term_two>`

Cada endpoint deve retornar um JSON contendo o resultado da operação na seguinte estrutura:

```json
{ "result": <int:result> }
```

Por exemplo:

```bash
GET /api/sub?term_one=4&term_two=1`
```

Deve retornar:

```json  
{ "result": 3 }
```
#### Requisitos

- A aplicação deve ser desenvolvida utilizando qualquer linguagem, mas daremos pontos extras para ruby, nodejs ou golang por serem mais aderentes à nossa stack.
- Pode-se utilizar qualquer framework para desenvolver a aplicação. Também pode-se optar por não utilizar nenhum framework.  
- Soluções que apresentem testes unitários e relatório de cobertura de testes receberão pontos extras.  
- A aplicação web deve rodar na porta `8000`.  
- A aplicação web deve ser acessível somente por outras aplicações dentro do cluster de kubernetes.
- A aplicação deve ter um healthcheck

### Deploy
Deverá ser possível fazer deploy da aplicação web no cluster de kubernetes. 

A documentação do projeto deverá mostrar como fazer o deploy da aplicação.

## Critérios de avaliação

 A solução do teste será avaliada segundo os requisitos propostos. Em um cluster nosso de Kubernetes, iremos:
 
- Fazer deploy da aplicação web seguindo a documentação entregue junto com o projeto.
- Validar se a API fornece os resultados esperados.
- Alterar a API e fazer um novo deploy.
- Remover a aplicação web seguindo a documentação entregue junto com o projeto.

Além disso, analisaremos o código do projeto entregue segundo algumas boas práticas de desenvolvimento, como:

- Facilidade para servir a aplicação, fazer deploy e remover a aplicação.
- Facilidade para dar manutenção na aplicação web.
- Legibilidade do código.
- Cobertura relevante de testes unitários.
