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

- Les relevés ont lieu à heures fixes, tous les intervalles depuis 06:05. Toutes les heures : à
  cinq. Toutes les 3 heures : 00:05, 03:05, 06:05 et ainsi de suite. Toutes les 6 heures : 00:05,
  06:05, 12:05 et 18:05. Toutes les 12 heures : 06:05 et 18:05. Chaque jour : 06:05. Le pied de la
  liste des sites et le menu de l'œil donnent le prochain.
- Un relevé complet de tous les sites, le vôtre ou un automatique, terminé moins d'un
  demi-intervalle avant une heure en tient lieu : relevez tout à 11:50 et, toutes les 6 heures, le
  prochain relevé automatique est celui de 18:05.
- Une heure manquée pendant que l'ordinateur dormait ou que Vigie était fermée donne un seul relevé
  dès que Vigie tourne à nouveau, pas un par heure manquée. Seules comptent les heures depuis que
  vous avez mis en marche les relevés automatiques : mis en marche à 10:00, toutes les 6 heures, le
  premier a lieu à 12:05 (tout de suite si aucun site n'a jamais été relevé).
- Un relevé prévu pendant qu'un autre a lieu attend sa fin. Un relevé automatique que vous arrêtez,
  ou qui ne va pas au bout, est retenté à l'heure suivante.
- Les relevés automatiques ont lieu tant que Vigie est ouverte : sa fenêtre ouverte, ou gardée
  ouverte dans la barre des menus.

## Notifications

Avec **Notifications** (le système vous le demande une fois), Vigie vous prévient quand un relevé
rend un site urgent, ou ne peut plus se connecter à un hébergement. Jamais pour ce qui était déjà
connu, jamais pour ce qui va mieux : une notification pour les sites devenus urgents, une pour les
hébergements devenus injoignables. Vigie vous prévient aussi quand un relevé trouve un nouvel
administrateur sur un site, ou une extension ou un thème installé hors de Vigie (voir
[security.md](security.md)).

Un relevé ou une mise à jour que vous avez lancé, de 30 secondes ou plus, dit aussi quand il se
termine, si la fenêtre de Vigie n'est pas devant (cachée, réduite, ou une autre app devant). Une
seule notification par exécution, par exemple « Relevé terminé » avec « Tous les sites : 12 sites
relevés, 1 urgent. », ce que l'exécution a aggravé dedans plutôt qu'à part. Jamais pour les
relevés automatiques, ni pour une exécution que vous avez arrêtée. Un clic ouvre la fenêtre sur ce
que l'exécution couvrait. Chaque notification est gardée sous son exécution dans **Activité**, la
fin d'une mise à jour de plusieurs sites sous son dernier site.

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
ExecStart=/usr/bin/vigie scan
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
