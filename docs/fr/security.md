# Relevés de sécurité : ce que Vigie lit, demande et envoie

Chaque relevé d'un site est aussi un relevé de sécurité. Il lit le site par SSH avec WP-CLI, comme
le reste du relevé, et ne fait que lire : rien n'est écrit sur le serveur. Certaines choses ne se
lisent pas sur le serveur : Vigie les demande alors à des services extérieurs, depuis votre
ordinateur, et seulement après votre accord. Cette page présente les deux, et la façon dont les
résultats sont classés.

## Ce qui est lu sur le serveur

Des commandes WP-CLI en lecture seule, sans charger les extensions ni les thèmes, et deux commandes
shell (le vrai dossier du site, et un `find` dans ses téléversements) :

- **Fichiers comparés avec wordpress.org** : `wp core verify-checksums` et
  `wp plugin verify-checksums --all`. Le serveur télécharge les sommes de contrôle publiées et lit
  chaque fichier : cela se fait donc **au plus une fois par jour** par site. Entre deux, la
  dernière comparaison est reprise. Elle se refait aussitôt quand WordPress ou une extension a
  changé de version, quand la dernière comparaison a échoué, ou quand vous la demandez
  (**Vérifier les fichiers**, ou `vigie scan --check-files`).
- **Fichiers PHP du dossier des téléversements**, où WordPress n'en met jamais : leurs noms et
  tailles, les 120 premiers octets d'un `index.php` et la première ligne d'un autre fichier. Cela
  suffit à reconnaître les fichiers que des extensions déposent exprès et qui n'exécutent rien : le
  `index.php` vide « Silence is golden » qui empêche de lister le dossier, celui qui ne répond que
  403 ou 404, et un fichier dont la première ligne arrête PHP (Matomo garde ainsi sa
  configuration). Rien de plus d'un fichier n'est lu, et rien n'en est gardé.
- **Administrateurs** (et super-administrateurs d'un réseau), avec les méthodes de double
  authentification que chacun a activées : Two Factor, Solid Security et WP 2FA dans les métadonnées
  de l'utilisateur, Wordfence dans sa propre table quand Wordfence est actif (cette lecture demande
  le client `mysql` sur le serveur). **Jamais** un secret de double authentification, un code de
  secours ou un mot de passe.
- **Réglages** : les noms des constantes de `wp-config.php`, puis les valeurs de `WP_DEBUG`,
  `WP_DEBUG_DISPLAY` et `DISALLOW_FILE_EDIT` seulement. **Jamais** le mot de passe de la base de
  données, les clés ni les sels.
- **Options** : si les moteurs de recherche sont priés de ne pas indexer le site, et la taille des
  options chargées sur chaque page (noms et tailles, jamais leurs valeurs).
- **WP-Cron** : les tâches planifiées et leur échéance (pas `wp cron test`, qui les exécuterait).
- **Version du serveur de base de données**, quand le client `mysql` est sur le serveur.

## Ce qui est demandé depuis votre ordinateur

Rien de ce qui suit n'est envoyé avant votre accord, donné sur l'écran affiché avant le premier
relevé qui en a besoin. Chaque service peut être coupé dans **Réglages, Recherches extérieures** :
ses vérifications sont alors « Non vérifié », jamais « À jour ».

| Service | Ce qu'il reçoit | Quand | Réponse gardée |
| --- | --- | --- | --- |
| [WPVulnerability](https://www.wpvulnerability.com/) | les slugs des extensions et des thèmes, la version de WordPress, la branche de PHP | à chaque relevé | un jour par slug ou version |
| api.wordpress.org | les slugs des extensions | à chaque relevé | un jour par slug |
| data.iana.org | rien sur vos sites (la liste des registres de domaines) | quand la liste gardée expire | jusqu'à son expiration |
| le registre du domaine de premier niveau (RDAP) | le nom de domaine enregistré, rien pour .ch, .li et .de, dont les registres ne publient pas de date d'expiration | à chaque relevé | une semaine par domaine |
| le site lui-même | son nom d'hôte (une connexion TLS sur le port 443, pour le certificat) et une requête du premier octet de `/wp-content/debug.log` | à chaque relevé | rien |

**Jamais envoyé** : les adresses de vos sites à un tiers, les versions de vos extensions et thèmes
(elles sont comparées sur votre ordinateur), vos accès SSH, rien de ce qui est lu dans vos bases de
données.

Chaque requête dit ce qu'elle est : `User-Agent: Vigie/<version> (+https://github.com/MarJC5/vigie)`.
Quand un service ne répond pas, la dernière réponse gardée sert, avec sa date. Sans réponse
gardée, la vérification est « Non vérifié ».

Les données de vulnérabilités viennent de WPVulnerability, dans le domaine public (CC0), sans
garantie : aucune vulnérabilité connue ne veut pas dire sûr. Vigie renvoie aux sources de chaque
fiche.

## Comment les constats sont classés

**Urgent** :

- une vulnérabilité de la version installée notée élevée ou critique (CVSS 3), connue pour être
  exploitée (CISA KEV), ou corrigée dans aucune version
- des fichiers de WordPress modifiés, manquants ou ajoutés (un seul constat pour tous, et
  `readme.html`, `license.txt` et `wp-config-sample.php` manquants sont seulement signalés : ils
  sont souvent supprimés exprès)
- un fichier PHP dans le dossier des téléversements, autre qu'un fichier déposé exprès qui
  n'exécute rien
- un certificat TLS expiré, pour un autre nom, d'une autorité inconnue, ou qui expire dans les
  7 jours
- un domaine qui expire dans les 7 jours
- un `debug.log` que tout le monde peut télécharger
- une extension fermée sur wordpress.org pour un problème de sécurité (seulement signalée quand ses
  fichiers montrent que l'extension installée en est une autre du même nom)

**À surveiller** :

- une mise à jour en attente
- toute autre vulnérabilité, même sans note
- une extension fermée pour une autre raison, ou sans nouvelle version depuis deux ans
- des fichiers d'une extension qui diffèrent de ceux de wordpress.org (un constat par extension).
  Quand presque tous diffèrent, c'est sans doute une autre extension du même nom. Quand aucun ne
  figure dans la liste de wordpress.org, ils n'ont pas pu être comparés, ce qui est seulement
  signalé.
- PHP qui ne reçoit plus que des correctifs de sécurité, ou qui n'est plus maintenu
- un administrateur sans double authentification
- la modification des fichiers permise depuis le tableau de bord
- des erreurs PHP affichées sur les pages
- le site masqué aux moteurs de recherche
- un site qui ne répond pas en HTTPS
- des tâches WP-Cron en retard de plus d'une heure
- des options chargées sur chaque page au-delà de 800 000 octets (la limite de WordPress lui-même)
- un certificat ou un domaine qui expire dans les 30 jours

**Information** : ce qui a été lu et va bien ou est bon à savoir : les administrateurs avec double
authentification, la version de la base de données, l'expiration du certificat et du domaine, les
extensions dont wordpress.org n'a pas les sommes de contrôle.

**Non vérifié** : une vérification qui n'a pas pu se faire, avec sa raison.

L'état d'un site est son pire constat non ignoré : **Urgent**, sinon **À surveiller**, sinon **Pas
entièrement vérifié** quand une vérification ne s'est pas faite, sinon **À jour**. Un relevé
annulé ou en échec après un relevé complet ne le remplace pas : le relevé complet reste affiché, et
la fenêtre dit pourquoi aucun plus récent ne l'est.

## Ignorer un constat

Depuis l'onglet Sécurité d'un site, **Ignorer** met un constat de côté, soit **jusqu'à la
prochaine version** de son extension, de son thème, de WordPress ou de PHP (cela s'annule tout seul
dès qu'un relevé lit une autre version), soit **définitivement**, avec une raison. Un constat
ignoré reste listé, grisé, avec sa raison et sa date, et ne compte plus dans l'état. **Ne plus
ignorer** le rétablit.

## En ligne de commande

`vigie scan [chemin]` se termine avec 0 quand chaque site est à jour, 1 quand un site est à
surveiller ou pas entièrement vérifié, 2 quand un site est urgent, et 3 quand un relevé n'a pas pu
aller au bout. `--json` liste les constats de chaque site avec leur niveau, leurs faits et s'ils
sont ignorés. `--check-files` compare à nouveau les fichiers, même dans la journée.
