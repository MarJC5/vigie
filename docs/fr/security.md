# Relevés de sécurité : ce que Vigie lit, demande et envoie

Chaque relevé d'un site est aussi un relevé de sécurité. Il lit le site par SSH avec WP-CLI, comme
le reste du relevé, et ne fait que lire : rien n'est écrit sur le serveur. Certaines choses ne se
lisent pas sur le serveur : Vigie les demande alors à des services extérieurs, depuis votre
ordinateur, et seulement après votre accord. Cette page présente les deux, et la façon dont les
résultats sont classés.

## Ce qui est lu sur le serveur

Des commandes WP-CLI en lecture seule, sans charger les extensions ni les thèmes, et trois
commandes shell (le vrai dossier du site, un `find` dans ses téléversements, et la fin de
`debug.log`) :

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
- **Les sous-sites d'un réseau** (multisite), en une fois : un script PHP en lecture seule livré
  avec Vigie, `internal/wpcli/subsites.php`, envoyé à `wp eval-file -` par l'entrée standard, pour
  que WordPress ne se charge qu'une fois pour tous les sous-sites. Il n'appelle que des fonctions
  de lecture de WordPress (un test en vérifie la liste) et imprime, pour chaque sous-site ni
  archivé, ni spam, ni supprimé, les extensions actives, le thème, l'e-mail d'administration et
  les administrateurs avec leurs méthodes de double authentification. Pour le réseau, les
  extensions actives partout et les super administrateurs. Chaque administrateur, du site principal
  ou d'un sous-site, devient alors un constat, avec les sous-sites qu'il administre, les super
  administrateurs en premier.
- **Réglages** : les noms des constantes de `wp-config.php`, puis les valeurs de `WP_DEBUG`,
  `WP_DEBUG_DISPLAY`, `WP_DEBUG_LOG` et `DISALLOW_FILE_EDIT` seulement. **Jamais** le mot de passe
  de la base de données, les clés ni les sels.
- **Options** : si les moteurs de recherche sont priés de ne pas indexer le site, la taille des
  options chargées sur chaque page (noms et tailles, jamais leurs valeurs), et l'e-mail
  d'administration avec celui qui attend sa confirmation (`admin_email`, `new_admin_email`), où
  WordPress envoie ses avis. Il reste sur votre ordinateur, comme les adresses des administrateurs.
- **PHP qui sert les pages**, qu'un hébergeur peut régler autrement que celui de la ligne de
  commande : le tableau de bord de WordPress interroge wordpress.org sur sa version de PHP et
  garde la réponse une semaine, sous une clé tirée de cette version (`wp_check_php_version()`).
  Vigie liste ces clés et leur expiration (`wp transient list --network --search=php_check_*`),
  jamais les réponses, et retrouve sur votre ordinateur la version de chacune. Elle n'est connue que
  si quelqu'un a ouvert le tableau de bord dans la semaine. Sinon, ou quand les notes sont dans un
  cache d'objets externe que WP-CLI ne sait pas lister, la fenêtre dit qu'elle n'est pas connue, et
  pourquoi. Quand elle est connue, les contrôles de PHP ci-dessous portent sur elle.
- **`debug.log`**, là où WordPress l'écrit (`WP_DEBUG_LOG`), ou celui resté d'avant dans
  `wp-content` : sa taille, et les erreurs fatales et d'analyse de son dernier mégaoctet, au plus
  50 lignes. Celles des 7 derniers jours donnent un constat avec leur nombre, la date de la dernière
  et son message, coupé à 300 caractères. Il peut contenir un chemin du serveur, et reste sur votre
  ordinateur.
- **WP-Cron** : les tâches planifiées et leur échéance (pas `wp cron test`, qui les exécuterait).
- **Version du serveur de base de données**, quand le client `mysql` est sur le serveur.
- **Les sauvegardes de Vigie**, faites avant chaque mise à jour : la date et la taille de celles de
  chaque mise à jour, dans `~/.vigie/backups/<identifiant du site>` (`du`), pour l'onglet Historique
  du site (voir [updates.md](updates.md)).

## Combien de temps prend un relevé, et ce qu'il demande au serveur

Un relevé lance environ 24 commandes sur le serveur, l'une après l'autre. La plupart prennent un
dixième de seconde. Le temps part dans ce que le serveur demande à wordpress.org : les vérifications
des mises à jour de WordPress, des extensions et des thèmes, quelques secondes chacune, et une fois
par jour la comparaison des fichiers, qui télécharge les sommes de contrôle de chaque extension.
Rien d'autre n'est demandé extension par extension : les traductions à mettre à jour et les
extensions que wordpress.org connaît viennent des vérifications des mises à jour, comme l'écran
Mises à jour de WordPress les montre. Sur le banc d'essai de Vigie, un site de 22 extensions est
relevé en 6 secondes environ, 25 secondes le jour où ses fichiers sont comparés.

- **Sites à la fois** : une série relève jusqu'à 16 sites à la fois, sur tous les hébergements.
- **Commandes à la fois sur un hébergement** : 2 par défaut, réglé dans la connexion de
  l'hébergement (**Commandes à la fois**). Les sites d'un hébergement se les partagent : 20 sites
  sur un même compte mutualisé prennent donc une à deux minutes. Gardez 2 en mutualisé, comme les
  hébergeurs s'y attendent. Sur votre propre serveur, vous pouvez monter jusqu'à 10, le nombre de
  commandes qu'un serveur SSH laisse tourner par défaut sur une connexion (`MaxSessions`). Deux
  hébergements de Vigie avec le même serveur, le même utilisateur et le même port partagent une
  connexion, et leurs commandes s'additionnent.
- **Connexions** : une connexion partagée par hébergement, gardée une minute après sa dernière
  commande (`ControlPersist`). Au plus 8 s'ouvrent en même temps, quels que soient les serveurs : un
  serveur SSH refuse des connexions ouvertes trop nombreuses à la fois (`MaxStartups`, 10 par
  défaut), et plusieurs hébergements peuvent être sur un même serveur.

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
| github.com | la version de Vigie, le système et l'architecture, pour apprendre une nouvelle version de Vigie (pas pour une installation par Homebrew ou un paquet Linux) | une fois par jour au plus, et quand vous le demandez dans **Réglages, À propos** | rien |

**Jamais envoyé** : les adresses de vos sites à un tiers, les versions de vos extensions et thèmes
(elles sont comparées sur votre ordinateur), vos accès SSH, rien de ce qui est lu dans vos bases de
données.

Chaque requête dit ce qu'elle est : `User-Agent: Vigie/<version> (+https://github.com/MarJC5/vigie)`.
Quand un service ne répond pas, la dernière réponse gardée sert, avec sa date. Sans réponse
gardée, la vérification est « Non vérifié ».

Les données de vulnérabilités viennent de WPVulnerability, dans le domaine public (CC0), sans
garantie : aucune vulnérabilité connue ne veut pas dire sûr. Vigie renvoie aux sources de chaque
fiche.

## WP-CLI installé sur un compte

Quand le test de connexion d'un hébergement trouve PHP mais pas WP-CLI, Vigie propose de
l'installer, dans l'assistant et dans **Tester la connexion**. Rien ne se passe avant votre
confirmation, et la fenêtre dit ce qui sera écrit. L'offre n'est pas faite quand PHP est plus
ancien que ce que demande WP-CLI (7.2.24), ni par-dessus un `~/bin/wp` déjà présent.

- **Ce qui est écrit** : un seul fichier, `~/bin/wp`, dans le dossier personnel du compte, rien
  dans vos sites. Il est écrit sous un nom temporaire puis renommé : un échec ne laisse rien.
- **D'où il vient** : de la version publiée par WP-CLI sur github.com, téléchargée **sur votre
  ordinateur**, pas par le serveur. Ce téléchargement est votre propre action, pas une des
  recherches ci-dessus : il n'envoie rien sur vos sites, et dit ce qu'il est
  (`User-Agent: Vigie/<version>`).
- **Comment il est vérifié** : Vigie installe une version de WP-CLI, fixée avec son empreinte
  SHA-512 ; un fichier qui ne correspond pas est écarté avant tout envoi. L'empreinte est vérifiée
  avec la signature GPG de WP-CLI à chaque changement de version.
- **Ensuite** : le fichier est envoyé par SSH et lancé une fois. La commande WP-CLI de
  l'hébergement devient `~/bin/wp` (ou `php ~/bin/wp` quand le dossier personnel n'autorise pas à
  lancer des programmes). Si aucune ne répond, le fichier est retiré.
- **Gardé dans Activité** : chaque installation, réussie ou arrêtée à une étape, sous son
  hébergement.

Pour le retirer : `rm ~/bin/wp` sur le compte. WP-CLI se met à jour avec `wp cli update` ; Vigie
ne le met pas à jour.

## Copies sur cet ordinateur

Une copie d'un site (voir [copies.md](copies.md)) ne fait que lire le serveur : un script lancé par
`wp eval-file`, qui dit où WordPress range ses parties, les tailles, la liste des fichiers, et les
archives lues par la connexion SSH. Rien n'est écrit sur le compte.

- **Ce qui quitte le serveur** : la base, et les fichiers que vous avez choisis, vers votre
  ordinateur seulement. Rien n'est envoyé ailleurs.
- **Ce qu'elle contient** : les données personnelles du site (comptes, commandes, messages) et,
  pour le site complet, `wp-config.php` avec le mot de passe de la base. Le dossier de chaque copie
  n'est lisible que par vous (0700, fichiers 0600). Le reste relève de la protection de votre
  ordinateur, avant tout un disque chiffré (FileVault sur macOS).
- **Combien de temps elle reste** : jusqu'à ce que vous la supprimiez. Vigie ne supprime jamais une
  copie d'elle-même, et une copie supprimée va dans la Corbeille.

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
- PHP qui ne reçoit plus que des correctifs de sécurité, ou qui n'est plus maintenu (celui qui
  sert les pages quand il est connu, celui de la ligne de commande sinon)
- un administrateur sans double authentification
- la modification des fichiers permise depuis le tableau de bord
- des erreurs PHP affichées sur les pages
- des erreurs fatales dans `debug.log` ces 7 derniers jours
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

## Ce qui a changé depuis le relevé précédent

Chaque relevé complet est comparé au relevé complet du site qui le précède (un relevé échoué ou
arrêté entre les deux est sauté), à partir de ce que les deux ont gardé : rien de plus n'est lu
sur le serveur. Est un changement :

- un administrateur ajouté (un super administrateur est signalé comme tel) ou retiré,
- une extension ou un thème installé ou retiré, ou le thème actif changé,
- une version de WordPress, d'une extension ou d'un thème qui a changé, **par Vigie** quand une de
  ses mises à jour du site l'a faite entre les deux relevés, **hors de Vigie** sinon (mises à jour
  automatiques, à la main, autre outil),
- des fichiers de WordPress ou d'une extension devenus différents de ceux de wordpress.org,
- un nouveau constat urgent ou à surveiller, non ignoré.

Ils sont listés sous chaque relevé dans le journal Activité, au-dessus de l'aperçu d'un site
(« Depuis le relevé précédent », absent quand rien n'a changé), et sous chaque site dans
`vigie scan`. Un nouvel administrateur, et une extension ou un thème installé hors de Vigie,
déclenchent une notification (voir [background.md](background.md)) : Vigie n'en installe jamais.
Un changement fait puis défait entre deux relevés n'est pas vu.

## En ligne de commande

`vigie scan [chemin]` se termine avec 0 quand chaque site est à jour, 1 quand un site est à
surveiller ou pas entièrement vérifié, 2 quand un site est urgent, et 3 quand un relevé n'a pas pu
aller au bout. `--json` liste les constats de chaque site avec leur niveau, leurs faits et s'ils
sont ignorés, et ses `changes` depuis le relevé précédent. `--check-files` compare à nouveau les fichiers, même dans la journée.

Dans un terminal, `vigie scan`, `vigie update` et `vigie discover` montrent leur avancement sur la
sortie d'erreur, redessiné sur place et effacé avant le résultat : une ligne par site en cours de
relevé avec la barre de ses étapes (par site en cours de mise à jour, avec son étape), les derniers
sites terminés, et un total avec le temps passé. Jamais plus haut que le terminal, jamais avec
`--json`, ni quand la sortie va dans un fichier ou un autre programme : les scripts lisent la même
sortie qu'avant.
