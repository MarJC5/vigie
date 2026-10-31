# Vigie en arrière-plan

Vigie peut veiller une fois sa fenêtre fermée : relever tous les sites à l'intervalle choisi, et
vous prévenir quand un relevé trouve quelque chose de nouveau et d'urgent. Chaque partie est coupée
tant que vous ne l'activez pas, dans **Réglages > Général > En arrière-plan**.

## Garder Vigie ouverte

Avec **Garder Vigie ouverte**, fermer la fenêtre laisse Vigie ouverte : elle quitte le Dock et garde
son œil dans la barre des menus (sous Linux, la zone de notification). Le menu de l'œil ouvre Vigie,
relève tous les sites, donne l'heure du prochain relevé automatique et le nombre de sites urgents,
et quitte. Quitter (⌘Q, ou le menu de l'œil) quitte toujours. Sans ce réglage, fermer la fenêtre
quitte Vigie, comme avant.

## Ouvrir à l'ouverture de session

Proposé dès que Vigie reste ouverte : Vigie démarre quand vous ouvrez votre session, fenêtre
fermée, dans la barre des menus. Sous macOS 13 et plus, c'est un élément d'ouverture du système
(Réglages Système > Général, éléments d'ouverture), où vous pouvez aussi le retirer. Vigie y lit son état et
n'en garde aucun. Sous Linux, c'est une entrée de `~/.config/autostart/`. Couper **Garder Vigie
ouverte** la retire.

## Relevés automatiques

Toutes les 1, 3, 6, 12 ou 24 heures, Vigie relève tous les sites, comme **Relever tous les sites**,
avec les mêmes limites par hébergement et votre choix pour les recherches extérieures. Un relevé
automatique ne met jamais rien à jour, et ne compare jamais à nouveau les fichiers (cela reste
**Vérifier les fichiers**).

- Le prochain relevé a lieu un intervalle après le dernier relevé complet de tous les sites, le
  vôtre ou un automatique, à l'heure près : le pied de la liste des sites et le menu de l'œil
  donnent son heure.
- Un ordinateur resté en veille plusieurs intervalles fait un seul relevé à son réveil, pas un par
  intervalle.
- Un relevé prévu pendant qu'un autre a lieu attend sa fin. Un relevé automatique que vous arrêtez,
  ou qui ne va pas au bout, est retenté un intervalle plus tard.
- Les relevés automatiques ont lieu tant que Vigie est ouverte : sa fenêtre ouverte, ou gardée
  ouverte dans la barre des menus.

## Notifications

Avec **Notifications** (le système vous le demande une fois), Vigie vous prévient quand un relevé
rend un site urgent, ou ne peut plus se connecter à un hébergement. Jamais pour ce qui était déjà
connu, jamais pour ce qui va mieux : une notification pour les sites devenus urgents, une pour les
hébergements devenus injoignables.

## Relevés avec Vigie fermée

Vigie n'installe rien dans le système. Pour relever avec Vigie fermée, demandez au planificateur de
votre système de lancer la ligne de commande, qui se termine avec 2 quand un site est urgent (voir
[security.md](security.md)). Utilisez-la à la place des relevés automatiques de Vigie, pas en plus :
deux programmes qui relèvent en même temps doubleraient la charge sur vos hébergements.

Sous macOS, un agent qui relève toutes les heures, enregistré sous
`~/Library/LaunchAgents/io.github.marjc5.vigie.scan.plist` :

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>io.github.marjc5.vigie.scan</string>
  <key>ProgramArguments</key>
  <array>
    <string>/Applications/Vigie.app/Contents/MacOS/Vigie</string>
    <string>scan</string>
  </array>
  <key>StartCalendarInterval</key>
  <dict>
    <key>Minute</key>
    <integer>0</integer>
  </dict>
</dict>
</plist>
```

puis `launchctl bootstrap gui/$(id -u) ~/Library/LaunchAgents/io.github.marjc5.vigie.scan.plist`.
Un intervalle de calendrier, contrairement à `StartInterval`, se lance une fois au réveil de
l'ordinateur si son heure est passée pendant la veille (`man launchd.plist`).

Sous Linux, un minuteur systemd de l'utilisateur : `~/.config/systemd/user/vigie-scan.service`

```ini
[Service]
Type=oneshot
ExecStart=/usr/local/bin/vigie scan
```

et `~/.config/systemd/user/vigie-scan.timer`

```ini
[Timer]
OnCalendar=hourly
Persistent=true

[Install]
WantedBy=timers.target
```

puis `systemctl --user enable --now vigie-scan.timer`.
