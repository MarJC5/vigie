# Copies de vos sites sur cet ordinateur

Vigie peut copier un site sur votre ordinateur : sa base de données, et si vous le voulez ses
fichiers. Elle lit le site par SSH et n'écrit rien sur le serveur. Une copie est à vous : Vigie n'en
supprime jamais d'elle-même, et n'en remet jamais une sur un serveur. Elle ne remplace pas les
sauvegardes de votre hébergeur.

## Ce que contient une copie

Vous choisissez à chaque copie, dans l'onglet **Copies** d'un site (ou **Copier sur cet
ordinateur** dans son menu) :

- **La base de données** : les articles, pages, réglages, comptes et commandes.
- **La base et wp-content** (par défaut) : plus les extensions, les thèmes et les médias. La fenêtre
  liste les dossiers de `wp-content` par taille. Décochez ceux dont vous ne voulez pas, comme un
  cache ou les archives d'une autre extension de sauvegarde.
- **Le site complet** : plus WordPress lui-même et `wp-config.php`, qui contient le mot de passe de
  la base.

Plusieurs sites sélectionnés se copient d'un coup, l'un après l'autre sur chaque hébergement, avec
le même contenu et sans dossier laissé de côté. Une copie contient les données personnelles du site
(comptes, commandes, messages) : gardez-la comme vous gardez le site.

## Où elle va

Dans **Réglages, Données**, le dossier des copies, `~/Documents/Vigie` par défaut. Chaque copie a
son dossier, `<hébergement>/<site>/<date et heure>`, lisible par vous seul. On y trouve les archives
et `copy.json`, qui dit ce que la copie contient : le site, sa version de WordPress et son préfixe de
tables, ce qui a été laissé de côté, et pour chaque archive sa taille, son empreinte SHA-256 et
l'heure où elle a été lue.

Avant de lancer une copie, Vigie compare sa taille à la place libre, et refuse quand elle ne tient
pas.

## Découpée en archives

Chaque partie du site a ses archives : `database.sql.gz`, `plugins.tar.gz`, `themes.tar.gz`,
`mu-plugins.tar.gz`, `uploads.tar.gz`, `others.tar.gz` (le reste de `wp-content`) et, pour le site
complet, `wordpress.tar.gz`. Une partie plus grande que la taille réglée dans **Réglages, Données**
(500 Mo par défaut, 4 Go au plus) est découpée : `uploads.tar.gz`, `uploads-2.tar.gz`, et ainsi de
suite. La base est découpée par tables entières.

Sur un compte sans GNU find ni GNU tar, chaque partie tient en une archive, non découpée.

## Une copie coupée

Une copie coupée (réseau, veille, **Arrêter**) garde ses archives complètes et s'affiche
**Incomplète**. **Reprendre** ne lit que les archives manquantes, les mêmes fichiers que la première
fois : le site a pu changer entre-temps, et `copy.json` garde l'heure de chaque archive. Une archive
supprimée à la main du dossier d'une copie compte aussi comme manquante.

## Supprimer une copie

L'icône de corbeille d'une copie la met à la Corbeille, d'où vous pouvez encore la récupérer. Quand
aucune Corbeille ne peut la recevoir (un disque réseau, un Linux sans bureau), Vigie le dit et
propose de la supprimer pour de bon, en redemandant.

## Remettre une copie en place à la main

Vigie ne remet pas une copie en place. Sur le compte du site, depuis le dossier de la copie envoyé
là-bas, chaque archive retourne dans le dossier de sa partie, et les exports dans la base, qu'ils
remplacent :

```sh
cd ~/www/mon-site                         # le dossier du site
gzip -dc database.sql.gz | wp db import -  # puis database-2.sql.gz et les autres, s'il y en a
tar -xzf plugins.tar.gz -C wp-content/plugins
tar -xzf themes.tar.gz -C wp-content/themes
tar -xzf uploads.tar.gz -C wp-content/uploads
tar -xzf others.tar.gz -C wp-content
```

Les archives `mu-plugins` vont dans `wp-content/mu-plugins`, les archives `wordpress` dans le
dossier du site. Pour une copie lancée ailleurs (un site local), importez l'export dans sa base,
puis changez l'adresse avec `wp search-replace`.

## En ligne de commande

```sh
vigie copy "Clients/Shared hosting/mon-site.example" --content database
vigie copy --tag clients --json
```

`--content` vaut `database`, `content` (par défaut) ou `site`. La commande ne demande rien avant,
puisqu'elle ne fait que lire les serveurs. Son code de sortie vaut 3 quand un site n'a pas pu être
copié.

## Ce qu'une copie lit sur le serveur

Quelques commandes en lecture seule donnent les tailles et les dossiers : un court script lancé par
`wp eval-file`, qui dit où WordPress range ses parties, `wp db size`, `du` et `find`. Puis les
archives : `wp db export` vers sa sortie standard (`--single-transaction`, pour lire les tables
InnoDB sans bloquer le site), et `tar` des fichiers listés, lus tous deux par la connexion SSH. Rien
n'est écrit sur le compte, et chaque copie compte dans les deux commandes à la fois que Vigie lance
par hébergement.
