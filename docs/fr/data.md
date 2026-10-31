# Données : ce que Vigie garde, et son format d'export

Vigie garde tout sur votre ordinateur, dans un seul fichier SQLite : `~/Library/Application
Support/Vigie/vigie.db` sous macOS, `~/.config/vigie/vigie.db` sous Linux (ou le dossier donné avec
`--data-dir`). À côté, `known_hosts` contient les clés des serveurs que vous avez approuvés. Avant
chaque changement du schéma de sa base, Vigie en écrit une copie, `vigie.db.before-<version>`.

## Le format d'export

Un export est un fichier JSON, en UTF-8, qui contient votre gestionnaire de sites. Vigie l'écrit,
Vigie le relit sur cet ordinateur ou sur un autre, et des add-ons peuvent aussi l'écrire ou le lire.
Sa forme exacte est le schéma JSON [`export-v1.schema.json`](../../addon/export/export-v1.schema.json).

Le format est défini une seule fois, dans le module Go `github.com/MarJC5/vigie/addon` (dossier
`addon/` de ce dépôt, versions marquées `addon/vX.Y.Z`) : Vigie est compilée avec lui, tout comme
un add-on écrit en Go, qui importe `github.com/MarJC5/vigie/addon/export` au lieu de recopier le
format. Un add-on écrit dans un autre langage lit le schéma JSON. Voir [addons.md](addons.md) pour
ce qu'un add-on répond.

```json
{
  "format": "vigie",
  "version": 1,
  "exported": "2026-10-30T14:12:00Z",
  "app": "0.6.0",
  "scope": "",
  "folders": [ ... ],
  "hosts": [ ... ],
  "sites": [ ... ],
  "ignores": [ ... ],
  "history": { "scans": [ ... ], "operations": [ ... ] }
}
```

* `scope` est le chemin du dossier ou de l'hébergement exporté (`Clients/Infomaniak`), ou `""` pour
  tout. Un dossier ou un hébergement au sommet de l'export a `""` pour parent.
* **folders** (dossiers) : `id`, `parent`, `name`, `color` (`none`, `red`, `green`, `blue`,
  `yellow`, `cyan`, `magenta`, `orange`), `notes` (Markdown), `position`.
* **hosts** (hébergements) : `id`, `folder`, `name`, et soit `alias` (une entrée de
  `~/.ssh/config`), soit `hostname`, `port` et `user`. Puis `keyPath` (le chemin d'une clé sur
  votre ordinateur, jamais la clé), `wpCli`, `maxConcurrent`, `color`, `notes`, `position`, et
  `trustedKeys` : les clés publiques du serveur que vous avez approuvées, chacune avec le `name` que
  lui donne le `known_hosts` de Vigie (`host` ou `[host]:port`), son `type` et sa `key`.
* **sites** : `id`, `host`, `name`, `path` (la racine de WordPress sur le serveur), `url`, `wpCli`,
  `color`, `notes`, `tags`, `position`, `multisite` et, pour un réseau, ses `subsites`.
* **ignores** (constats ignorés) : les constats mis de côté : `site`, la `key` du constat,
  `untilVersion` (`""` quand c'est définitif), `reason`, `created`.
* **history** (historique), seulement si vous le demandez : les relevés des sites exportés, avec ce
  qu'ils ont lu (`items`) et trouvé (`findings`), et les mises à jour et retours en arrière avec
  leurs étapes.

**Jamais dans un export** : les clés privées, les mots de passe, les réglages de cet ordinateur
(langue, votre accord pour les recherches qui en partent), les réponses que Vigie garde des
services extérieurs.

**Sensible** : un export nomme vos serveurs, vos utilisateurs SSH et contient vos notes. Vigie
l'écrit lisible par vous seul (`0600`), par un fichier temporaire renommé à la fin : un export
interrompu ne laisse jamais un demi-fichier. Gardez-le en lieu sûr.

### Versions

`version` change quand le format change. Vigie lit toutes les versions plus anciennes, et refuse une
version plus récente avec un message, plutôt que de laisser tomber ce qu'elle ne connaît pas : un
champ inconnu est refusé pour la même raison.

Quand Vigie lit un export, elle vérifie aussi qu'il se tient : chaque `id` est un identifiant,
chacun une seule fois, chaque `parent`, `folder`, `host` et `site` est dans le fichier, les noms
sont donnés, aucun dossier n'est dans lui-même, et couleurs, types de clés et niveaux sont parmi
les valeurs que liste le schéma.

## Importer

Un import ajoute ce qui est nouveau, dans le dossier que vous choisissez (le sommet par défaut), et
ne change rien de ce qui est déjà là. Avant d'ajouter quoi que ce soit, il montre ce qu'il fera de
chaque dossier, hébergement et site du fichier :

* **déjà là**, laissé tel quel : le même identifiant. Pour un dossier, un dossier du même nom au
  même endroit, qui reçoit ce que le fichier y met. Pour un hébergement, un hébergement avec le même
  alias, ou les mêmes nom d'hôte, port et utilisateur, qui reçoit les nouveaux sites du fichier.
  Pour un site, un site au même chemin sur le même hébergement.
* **ajouté**, avec l'identifiant du fichier, ou un nouveau quand un autre élément l'a déjà, et
  `(2)`, `(3)`... après son nom quand un voisin porte ce nom.
* **refusé**, avec la raison, et avec lui ce qu'il contient.

Les constats ignorés et l'historique du fichier ne viennent qu'avec les sites ajoutés. Une mise à
jour ou un retour en arrière encore en cours là où le fichier a été écrit revient comme interrompu.

**Les clés d'hôte approuvées** ne reviennent qu'une fois confirmées : l'aperçu montre l'empreinte de
chaque clé, pour l'hébergement auquel elle appartient. Une clé que vous laissez de côté, ou que
Vigie ne peut pas prendre (pas pour ce serveur, ou Vigie approuve déjà une autre clé pour lui),
n'est pas gardée, et l'hébergement la demande à sa première connexion, comme un nouvel hébergement.

Tout est ajouté d'un coup : si quelque chose a changé entre-temps et contredit l'aperçu, rien n'est
ajouté.

## En ligne de commande

```sh
vigie export [chemin] [--history] [-o fichier] [--json]
vigie import <fichier> [--into dossier] [--yes] [--trust-keys] [--json]
vigie import --ssh-config [alias...] [--into dossier] [--yes] [--json]
```

* `export` exporte tout, un dossier ou un hébergement. Avec `-o`, il écrit le fichier, lisible par
  vous seul, et dit ce qu'il contient (`--json` : en JSON). Sans `-o`, il écrit l'export lui-même
  sur la sortie standard, pour un script ou un add-on.
* `import` lit un export, ou une liste de sites CSV (par son extension `.csv`, ou quand le fichier
  n'est pas un export), ou avec `--ssh-config` les alias que vous nommez de `~/.ssh/config`. Ce
  fichier nomme n'importe quel serveur, pas seulement ceux de sites WordPress : sans alias,
  `--ssh-config` liste ses noms avec leur compte, et ceux qu'un hébergement atteint déjà, et
  n'ajoute rien. Il affiche l'aperçu, demande avant d'ajouter quoi que ce soit, puis demande pour
  chaque clé d'hôte. `--yes` répond à la première question pour un script : les clés restent alors
  de côté, sauf si `--trust-keys` les garde toutes. `--json` demande `--yes` et affiche l'aperçu
  avec `"imported": true` une fois fait.
* Code de sortie : 0 fait, ou rien à importer, 1 quand quelque chose a été refusé ou laissé de
  côté, ou que vous avez répondu non, 3 en cas d'erreur.

## Importer une liste de sites (CSV)

Une liste de sites, tirée d'un tableur par exemple, s'importe de la même façon : Vigie en fait un
export avec de nouveaux identifiants, montre l'aperçu, et ajoute ce que vous confirmez.

* **Une ligne d'en-tête** nomme les colonnes, dans n'importe quel ordre : `path` (la racine de
  WordPress sur le serveur), et soit `alias` (une entrée de `~/.ssh/config`), soit `host` et `user`
  avec un `port` facultatif. Puis, si vous voulez, `folder`, `url`, `name`, `color` et `notes`. Une
  autre colonne fait refuser le fichier.
* **Une ligne par site.** Les valeurs sont séparées par des virgules, ou par des points-virgules
  quand l'en-tête en contient davantage, comme les écrivent les tableurs en français. Des guillemets
  gardent un séparateur ou un retour à la ligne dans une valeur (RFC 4180). Une marque d'ordre des
  octets au début est ignorée.
* **Les dossiers** sont des chemins (`Clients/Valais`), créés s'ils manquent. Les sites d'un même
  compte (le même alias, ou les mêmes hôte, port et utilisateur) partagent un hébergement, nommé
  d'après son alias ou son hôte.
* Un site sans `name` prend le nom de son adresse, sinon de la dernière partie de son chemin.
  `color` est l'une de `none`, `red`, `green`, `blue`, `yellow`, `cyan`, `magenta`, `orange`.

```csv
folder;host;port;user;path;url
Clients/Valais;ssh.example-host.ch;22;u12ab;~/sites/atelier;https://atelier-exemple.ch
Clients/Valais;ssh.example-host.ch;22;u12ab;~/sites/cave;https://cave-exemple.ch
```

Une ligne que Vigie ne peut pas prendre est listée avec son numéro et la raison : pas de chemin,
pas de connexion ou les deux, un hôte sans utilisateur, un port, une couleur ou un nom illisible, un
compte déjà placé dans un autre dossier sur une ligne précédente, le même site deux fois, ou pas
autant de valeurs que l'en-tête.
