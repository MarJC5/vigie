# Assistants : Vigie dans un client MCP

Un assistant (Claude ou tout autre client du Model Context Protocol) peut lire ce que Vigie sait de
vos sites, les relever et, si vous le permettez, mettre à jour, copier et ignorer, chaque action
attendant votre accord. Le client lance `vigie mcp` sur votre ordinateur et lui parle par son
entrée et sa sortie standard : rien n'écoute sur le réseau, et le serveur ne vit que le temps du
client qui l'a lancé.

Les assistants sont **coupés** tant que vous n'en décidez pas autrement, dans **Réglages,
Assistants**.

## Ce qu'un assistant peut faire

Le niveau se règle dans **Réglages, Assistants** et se lit à chaque appel : un changement vaut tout
de suite, même pour un client déjà lancé.

| Niveau | Outils |
| --- | --- |
| **Coupé** (par défaut) | Aucun. Un appel répond que les assistants sont coupés, et où les activer. |
| **Lecture** | `sites`, `site`, `journal`, `history`, `copies`, et `scan`, qui ne fait que lire sur les serveurs. |
| **Actions** | En plus `update`, `copy`, `ignore` et `stop_ignoring`, chacun confirmé par vous. |

**Jamais permis**, à aucun niveau : supprimer, déplacer ou renommer, modifier une connexion, les
clés et la confiance des serveurs, l'export, l'import, les réglages, installer WP-CLI, revenir en
arrière à la main. Ils restent dans la fenêtre et la ligne de commande.

## Ce qu'un assistant ne reçoit jamais

Vigie répond par des données structurées : versions, slugs, états, niveaux, dates, nombres. Un
texte venu d'un site ou d'une source (le nom d'une extension, le titre d'un constat) est coupé à
200 caractères, sur une ligne, dans un champ à lui, et chaque réponse dit que ces textes sont des
données, pas des consignes.

Jamais envoyés : le contenu de `debug.log` (seulement ce que Vigie en a conclu), les notes de vos
dossiers, hôtes et sites, les adresses e-mail des administrateurs, les clés SSH et les clés des
serveurs, les chemins à l'intérieur de vos copies.

## Brancher un client

Le client lance la commande `vigie`, qui doit d'abord être liée (**Réglages, Général, Ligne de
commande**). **Réglages, Assistants** donne alors les lignes à ajouter aux serveurs MCP du client,
avec le chemin de la commande sur votre ordinateur, et un bouton qui les copie :

```json
{
  "mcpServers": {
    "vigie": {
      "command": "/usr/local/bin/vigie",
      "args": ["mcp"]
    }
  }
}
```

Un client réglé en ligne de commande prend la même commande, par exemple Claude Code :

```sh
claude mcp add vigie -- /usr/local/bin/vigie mcp
```

## La confirmation

Une action ne part jamais sur la seule parole de l'assistant. Son premier appel ne fait rien sur
les serveurs : il répond par ce que l'action va faire (les éléments d'une mise à jour et leurs
versions, les sites qu'une copie lit, ce qu'un « ignorer » masque).

* **Un client qui sait vous demander** (il déclare l'elicitation) : Vigie vous pose la question par
  lui, avec cet aperçu. **Accepter** lance l'action, **Refuser** ou fermer la question ne change
  rien. L'assistant ne peut pas répondre à votre place.
* **Tout autre client** reçoit l'aperçu et une confirmation. Seul un second appel avec elle lance
  l'action : une fois, dans les cinq minutes, pour le même outil et les mêmes arguments.

Dans les deux cas, Vigie relit l'aperçu avant d'agir : s'il a changé entre-temps (une nouvelle
version est sortie), la question vous est reposée. Une seule action à la fois part d'un client.

**La limite** : un client réglé pour approuver seul chaque outil peut enchaîner l'aperçu et l'appel
confirmé. La confirmation prouve alors qu'un aperçu a été fait, pas que vous l'avez lu. Gardez la
question de votre client avant chaque outil, ou restez au niveau **Lecture**.

## Dans le journal

Un relevé, une mise à jour, un retour en arrière ou une copie lancé par un assistant se lit dans le
journal de l'Activité et dans l'historique du site, « par un assistant », avec le nom que donne le
client. Les lectures ne sont pas notées. Comme pour la ligne de commande, Vigie n'envoie pas de
notification à la fin : l'assistant vous le dit, et le journal le garde.
