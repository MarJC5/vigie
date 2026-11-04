# Importer et exporter

Vos dossiers, hébergements et sites peuvent être enregistrés dans un fichier, passer sur un autre
ordinateur, ou venir d'ailleurs. Rien n'est envoyé nulle part.

## Exporter

**Réglages, Données, Exporter**, ou **Exporter** dans le menu d'un dossier ou d'un hébergement.
Le fichier n'est lisible que par vous. Il nomme vos serveurs et vos utilisateurs SSH et contient vos
notes : gardez-le en lieu sûr. **Jamais** une clé privée ni un mot de passe.

## Importer

Dans **Réglages, Données** :

- **Un fichier Vigie** : un export, de cet ordinateur ou d'un autre.
- **Un fichier CSV** : une ligne par site, tirée d'un tableur par exemple.
- **Configuration SSH** : les hôtes de `~/.ssh/config` que vous choisissez.

Un aperçu montre ce qui sera ajouté. Seul ce qui est nouveau est ajouté, rien d'existant n'est
modifié. Les clés de serveur qu'apporte un fichier ne sont gardées qu'une fois confirmées.

## Depuis un autre programme

Un add-on amène les données d'un autre programme, par exemple le gestionnaire de sites de
FileZilla. Installez-le, puis ouvrez sa page dans **Add-ons** : ce qu'il a écrit s'ouvre dans le
même aperçu.

Pour aller plus loin : [le format d'export et les colonnes du CSV](../data.md).
