# Mises à jour : ce que Vigie fait sur un serveur

Vigie met à jour un site par SSH avec WP-CLI, comme pour les relevés. Rien n'est installé sur le
site. Cette page dit ce qu'une mise à jour écrit sur le serveur, combien de temps elle le garde, et
comment le supprimer.

## Avant d'écrire quoi que ce soit

- Vigie lit d'abord l'aperçu : les mises à jour que propose WordPress, extensions chargées, pour que
  les extensions payantes montrent leurs propres mises à jour.
- Le serveur doit avoir un outil de sauvegarde de la base de données (`mariadb-dump` ou
  `mysqldump`). Sans lui, rien n'est mis à jour : Vigie sauvegarde la base avant chaque mise à
  jour.
- Une seule mise à jour ou un seul retour en arrière à la fois sur un site.
- Une mise à jour peut être **ignorée** depuis l'onglet Mises à jour, jusqu'à ce qu'une version plus
  récente soit proposée, ou définitivement avec une raison (une version que vous gardez exprès).
  Elle ne compte alors plus nulle part (état, compteurs, notifications), **Tout mettre à jour** et
  `vigie update` la laissent de côté et le disent, et la mise à jour du site ne l'installe que si
  vous la cochez. Les mises de côté restent sur votre ordinateur, avec les autres.
- Chaque mise à jour proposée renvoie à son changelog quand wordpress.org en a un : l'onglet
  Development d'une extension, les notes de version de WordPress. Votre navigateur l'ouvre, Vigie
  ne télécharge rien.

## Les étapes d'une mise à jour

1. **Sauvegardes**, dans un nouveau dossier du compte (voir plus bas) : la sauvegarde de la base,
   puis une archive du dossier de chaque extension ou thème à mettre à jour, et des fichiers de
   WordPress quand WordPress lui-même est mis à jour.
2. **WordPress lui-même** d'abord, s'il est choisi : en mode maintenance (le fichier `.maintenance`
   de WordPress, par WP-CLI), puis la mise à niveau de sa base. Le mode maintenance est toujours
   retiré à la fin.
3. **Les extensions, puis les thèmes**, un par un.
4. **Les traductions** en dernier.

Après chaque élément, Vigie **vérifie le site** : la page d'accueil, `wp-login.php` et les
sous-sites épinglés d'un réseau ne doivent répondre ni une erreur du serveur (5xx), ni la page
d'erreur critique de WordPress, ni rien du tout, et aucune nouvelle `PHP Fatal error` ne doit
apparaître dans `debug.log` (quand WordPress en tient un). Les redirections et les réponses 4xx
passent. La première vérification a lieu 3 secondes après l'élément, puis une chaque seconde
pendant 3 secondes, le temps que l'OPcache de PHP serve le nouveau code.

La mise à jour **s'arrête au premier échec** : les éléments suivants ne sont pas commencés. Arrêter
depuis la fenêtre ou avec Ctrl-C laisse finir l'élément en cours : un élément n'est jamais laissé à
moitié.

## Ce que Vigie écrit sur le serveur

Les sauvegardes d'une mise à jour vont dans un dossier du dossier personnel du compte SSH, hors du
dossier du site :

```
~/.vigie/backups/<identifiant du site>/<date et heure, UTC>/
    database.sql.gz          toute la base de données, compressée
    plugin-<slug>.tar.gz     le dossier de chaque extension mise à jour
    theme-<slug>.tar.gz      le dossier de chaque thème mis à jour
    core.tar.gz              les fichiers de WordPress, quand WordPress est mis à jour
```

- Le dossier est créé lisible par le compte seul (`umask 077`).
- `core.tar.gz` laisse de côté `wp-content`, `wp-config.php` (qui contient le mot de passe de la
  base) et `.maintenance`.
- **La sauvegarde contient toute la base de données** : les comptes avec leurs mots de passe
  chiffrés, les commandes, les envois de formulaires. Si le dossier personnel du compte est lui-même
  servi sur le web, déplacez les sauvegardes ou supprimez-les (plus bas).
- L'identifiant du site est celui que Vigie lui a donné. La fenêtre montre le dossier dans l'aperçu
  d'une mise à jour et dans l'historique du site.

En dehors des sauvegardes, une mise à jour n'écrit que ce que la mise à jour elle-même change (les
fichiers des éléments, la mise à niveau de la base par WordPress ou une extension) et, pour une
mise à jour de WordPress, le fichier `.maintenance` pendant qu'elle a lieu.

## Combien de temps les sauvegardes sont gardées

Vigie garde les sauvegardes d'un site **30 jours**, et au plus celles de ses **3 dernières mises à
jour**. La durée se règle dans **Réglages, Données** : 7, 30 ou 90 jours, ou sans limite (seulement
les 3 dernières mises à jour alors). Les sauvegardes contiennent les données des visiteurs et des
clients du site : elles ne sont pas gardées plus longtemps que nécessaire.

À la fin de chaque mise à jour, même arrêtée, Vigie supprime les dossiers au-delà de ces limites,
pour tous les sites de ce Vigie sur le même hébergement, pas seulement celui mis à jour : une liste
et les suppressions, dans son propre dossier seulement. Un dossier au nom d'un site que ce Vigie ne
connaît pas reste tel quel : il peut appartenir au Vigie d'un autre ordinateur.

Chaque relevé lit les sauvegardes d'un site, la date et la taille de celles de chaque mise à jour,
et ne fait que lire. L'onglet **Historique** du site les liste, et celles au-delà des limites se
suppriment d'un coup avec **Supprimer**, après une confirmation. L'historique des mises à jour
reste dans la base de Vigie, sur votre ordinateur.

## Revenir en arrière

Un retour en arrière est proposé sur la dernière mise à jour d'un site, quand elle a échoué :

- **Une extension ou un thème** : son dossier est supprimé, puis remis depuis son archive, et le
  site est vérifié à nouveau. La base de données n'est pas touchée : une extension qui a modifié ses
  tables pendant la mise à jour les garde.
- **WordPress lui-même** : ses fichiers et la base sont remis tels qu'avant la mise à jour, en mode
  maintenance. Ce qui a été écrit dans la base entre-temps (commandes, commentaires) est perdu.

Seul l'élément en échec est remis : les éléments mis à jour avant lui gardent leur nouvelle
version.

## Supprimer les sauvegardes

La suppression d'un site ou d'un hébergement dans Vigie propose, cochée, de supprimer aussi les
sauvegardes de ses sites sur le serveur. Si le serveur ne répond pas, le site est quand même
supprimé de Vigie, et Vigie donne la commande à lancer sur le serveur.

Les sauvegardes sont des fichiers ordinaires : vous pouvez aussi les supprimer par SSH.

```sh
rm -rf ~/.vigie/backups/<identifiant du site>   # les sauvegardes d'un site
rm -rf ~/.vigie                                 # tout ce que Vigie a écrit sur ce compte
```

Un retour en arrière n'est alors plus possible pour ces mises à jour.
