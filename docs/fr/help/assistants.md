# Assistants

Un assistant, Claude ou tout autre client MCP, peut lire ce que Vigie sait de vos sites, et agir
dessus si vous le permettez. Le client lance `vigie mcp` sur cet ordinateur : rien n'écoute sur le
réseau.

## Choisir ce qu'il peut faire

Dans **Réglages, Assistants** :

* **Coupé**, par défaut : Vigie ne répond à aucun assistant.
* **Lecture** : les sites, le journal, l'historique, les copies, et un relevé, qui ne fait que lire
  sur les serveurs.
* **Actions** : en plus mettre à jour, copier sur cet ordinateur, et ignorer.

Supprimer, déplacer, les connexions et les clés des serveurs, l'export, l'import et les réglages ne
sont jamais permis. Le contenu de `debug.log`, vos notes et les adresses e-mail des administrateurs
ne sont jamais envoyés. Ce qu'il lit, adresses de vos sites comprises, va en général au fournisseur
de son modèle.

## Brancher un client

Liez d'abord la commande `vigie`, dans **Réglages, Général, Ligne de commande**. **Réglages,
Assistants** donne alors les lignes à ajouter aux serveurs MCP de votre client : copiez-les avec le
bouton. Chaque client les garde dans un fichier à lui :
[où, client par client](../assistants.md#brancher-un-client).

## Votre accord avant chaque action

Une action répond d'abord ce qu'elle va faire, et rien de plus. Un client qui sait vous demander
affiche la question de Vigie : **Accepter** la lance, **Refuser** ne change rien. Un autre client
reçoit une confirmation qui vaut une fois, dans les cinq minutes. Gardez la question de votre client
avant chaque outil : un client réglé pour tout approuver seul pourrait approuver à votre place.

Ce qu'un assistant a lancé s'affiche dans le journal, « par un assistant ».

Pour aller plus loin : [les niveaux, la confirmation et sa limite](../assistants.md).
