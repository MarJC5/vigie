# Add-ons

Un add-on est un programme à part, nommé `vigie-<nom>`, que vous installez comme n'importe quel
autre (par exemple avec Homebrew) et que Vigie trouve dans votre `PATH`. Il amène des données dans
Vigie (une source, qui écrit un export de Vigie) ou emporte ailleurs ce que Vigie écrit (une
destination). Vigie n'en télécharge jamais, ne lui donne ni données ni accès à quoi que ce soit, et
ne le lance que quand vous le démarrez.

## Comment Vigie les trouve et les lance

* **En ligne de commande**, `vigie <nom> [arguments]` lance `vigie-<nom>` trouvé dans votre `PATH`
  avec les arguments tels quels, et se termine avec son code de sortie, comme Git lance ses
  commandes externes.
* **Les add-ons installés** sont les programmes nommés `vigie-<nom>` dans les dossiers de votre
  `PATH`, puis dans les dossiers d'installation habituels qui existent (`/opt/homebrew/bin`,
  `/usr/local/bin`, `~/.local/bin`, `~/go/bin` : une app ouverte depuis le Finder ou le Dock ne
  reçoit pas le `PATH` de votre shell), le premier de chaque nom dans cet ordre. Un dossier qui
  n'est pas absolu est ignoré : un programme du dossier courant n'est jamais lancé. Chacun est
  interrogé sur sa description (`--vigie-info`, dix secondes au plus).
* **Le lancement d'une source** reçoit `--vigie-export` avec un fichier dans un dossier à lui,
  lisible par vous seul, puis chaque réglage comme argument séparé, jamais par un shell. Il est
  arrêté au bout de deux minutes. Son export est lu, montré dans l'aperçu d'import, puis supprimé.
* **Dans la fenêtre**, l'élément Add-ons de la barre montre deux onglets : la Bibliothèque et les
  add-ons installés. Chaque add-on a sa page : une source s'y lance avec ses réglages, et ce qu'elle
  a écrit s'ouvre dans l'aperçu d'import, où rien n'est ajouté avant votre confirmation.
* **La Bibliothèque** liste les add-ons connus, installés ou non, d'après un fichier livré avec
  chaque version de Vigie (`internal/addons/library.json`, produit par `make library` à partir du
  `--vigie-info` des add-ons eux-mêmes). Vigie ne télécharge ni n'installe jamais un add-on : la
  page d'un add-on non installé dit où le trouver.

## Add-ons connus

| Add-on | Type | Ce qu'il fait |
| --- | --- | --- |
| [`vigie-filezilla`](https://github.com/MarJC5/vigie-filezilla) | source | Fait du gestionnaire de sites de FileZilla des hébergements de Vigie : ses dossiers, ses serveurs SFTP avec utilisateur, port, fichier de clé, commentaires et couleur. Ne lit jamais les mots de passe. |

## Inscrire un add-on dans la Bibliothèque

Le fichier de la Bibliothèque est produit, jamais modifié à la main. Ajoutez à
`internal/addons/library.json` une entrée avec seulement l'adresse d'installation et le nom de
l'add-on :

```json
{"install": "https://github.com/someone/vigie-example", "info": {"name": "example"}}
```

puis installez une version publiée de l'add-on (pas une version de développement) et lancez
`make library` : chaque entrée prend la description que son add-on affiche avec `--vigie-info`,
sans ses réglages. Relancez-le quand un add-on change ses textes ou son image.

## Écrire un add-on

Un add-on répond à deux options, réservées à Vigie. Toutes les autres options sont les siennes.

* `--vigie-info` affiche sa description en JSON sur la sortie standard, et se termine avec 0. Vigie
  en tire la page de l'add-on : son titre, son auteur, sa version, son type, une phrase pour sa
  carte, le texte de sa page, des précisions (ce qu'il lit, écrit, ce à quoi il se connecte), une
  image, et les réglages que demande un lancement. Sa forme exacte est le schéma JSON
  [`info-v1.schema.json`](../../addon/info/info-v1.schema.json).
* `--vigie-export <chemin>`, pour une source, écrit un export de Vigie à ce chemin, lisible par son
  propriétaire seul, puis se termine avec 0. Ce qu'il écrit sur sa sortie d'erreur est montré à
  l'utilisateur mot pour mot, comme son compte rendu. Vigie arrête un lancement au bout de deux
  minutes. La forme de l'export est décrite dans [data.md](data.md).

**Les textes** sont donnés en anglais (`"en"`), et en français (`"fr"`) quand l'add-on les a. Vigie
montre la langue de l'utilisateur, sinon l'anglais.

**L'image** est une URI de données d'un PNG ou d'un SVG, de 256 Ko au plus une fois décodée. Vigie
la montre comme une image, où un SVG n'exécute aucun script et ne charge rien. Sans image, l'add-on
montre l'image par défaut de Vigie.

**Les réglages** : un fichier, un texte, une case à cocher ou un choix, chacun avec son libellé et
l'option sous laquelle il est passé. Vigie passe chaque valeur comme argument séparé, après
l'option (`--file /chemin/du/fichier`), jamais par un shell. Une case à cocher est son option
seule, quand elle est cochée. Un réglage non modifié prend sa valeur par défaut.

## En Go

Le module `github.com/MarJC5/vigie/addon` (dossier `addon/` de Vigie, versions marquées
`addon/vX.Y.Z`) contient la seule définition de ce qu'échangent Vigie et ses add-ons :

* `addon/export` : le format d'export (types, `Write`, `WriteFile`, `Read`, `Check`)
* `addon/info` : la description d'un add-on (types, `Write`, `Read`, `Check`, les deux options)
* `addon/addontest` : les vérifications qu'un add-on fait dans ses propres tests

```go
func TestTheContract(t *testing.T) {
	program := addontest.Build(t, "./cmd/vigie-example")
	addontest.Info(t, program) // the description reads back and names the program
	f, report := addontest.Export(t, program, map[string]string{"file": "testdata/list.xml"})
	// the export holds together and is readable by its owner only
}
```

Un add-on qui ne correspond plus au format de Vigie échoue à la compilation ou à ces vérifications,
plutôt qu'à l'import de l'utilisateur.
