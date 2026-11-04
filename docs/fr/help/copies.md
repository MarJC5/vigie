# Copies

Vigie copie un site sur cet ordinateur : sa base de données, et si vous le voulez ses fichiers. Elle
ne fait que lire le serveur. Une copie est à vous, et elle ne remplace pas les sauvegardes de votre
hébergeur.

## Copier un site

Dans l'onglet **Copies** du site, **Copier**, ou **Copier sur cet ordinateur** dans son menu.
Choisissez la base, la base et `wp-content` (par défaut), ou le site complet. Décochez les dossiers
de `wp-content` dont vous ne voulez pas, comme un cache. Plusieurs sites sélectionnés se copient
d'un coup.

Les copies vont dans le dossier réglé dans **Réglages, Données**, `~/Documents/Vigie` par défaut, un
dossier par site et par date. Une grande partie est découpée en archives de la taille réglée là.

## Coupée, supprimée

Une copie coupée s'affiche **Incomplète** : **Reprendre** ne lit que ce qui manque. L'icône de
corbeille met une copie à la Corbeille. Une copie contient les données personnelles du site :
gardez-la comme vous gardez le site.

## La remettre en place

Vigie ne remet jamais une copie sur un serveur. À la main, sur le compte du site : chaque archive
retourne dans le dossier de sa partie avec `tar -xzf`, la base avec `wp db import`.

Pour aller plus loin : [ce que contient une copie, et comment la remettre en place](../copies.md).
