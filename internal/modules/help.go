package modules

const HelpText = `<b>AniToon_4Bot — Group Management</b>

<b>General commands</b>
/start — introduction
/help — command guide
/id — show chat and user IDs

<b>Clone setup (main bot private chat)</b>
/clone — connect your own BotFather bot token
/cancel — cancel clone setup

<b>Moderation</b>
/ban — ban a member (reply or numeric user ID)
/unban — remove a ban (numeric user ID)
/kick — remove a member (they may rejoin)
/mute — mute permanently or for a duration
/unmute — restore Telegram chat permissions
/warn — add a warning (reply or numeric user ID)
/warnings — view warning history for a member
/resetwarn — clear a member's warnings
/lock [type] — lock a permission category (defaults to all)
/unlock [type] — restore a category or all original permissions
/locks — list active group locks
/setflood <limit> <seconds> [delete|mute|ban] — configure anti-flood
/flood — show anti-flood settings
/clearflood — disable anti-flood
/setraid <joins> <seconds> [minutes] — configure anti-raid
/raid — show anti-raid settings
/clearraid — disable anti-raid
/slowmode [seconds|off] — set or disable member message interval
/antilink [on|off] — block unapproved links and Telegram invites
/allowlink add|remove <domain> — manage trusted domains
/allowlink list — show trusted domains
/antiforward [on|off] — block forwarded messages from regular members
/addblacklist <phrase> — block a word or phrase
/unblacklist <phrase> — remove a blacklist phrase
/blacklist — list blocked phrases
/blacklistmode [delete|warn] — choose the action
/welcome [on|off] — enable or show welcome messages
/setwelcome <text> — set a welcome template
/resetwelcome — restore the default welcome template
/goodbye [on|off] — enable or show goodbye messages
/setgoodbye <text> — set a goodbye template
/resetgoodbye — restore the default goodbye template
/setrules [rules text] — set this group's rules (admin only)
/rules — display this group's rules
/privaterules — send the rules to your private chat
/clearrules — remove this bot's rules for this group (admin only)

/modlog [count] — show recent moderation actions (1–20)

Moderation commands require you to be a group administrator. The bot must have the relevant administrator permissions. Warning history is stored per group in PostgreSQL. Three warnings trigger an admin review notice only; there is no automatic punishment. Clone setup is available only in the main AniToon bot's private chat. Anonymous-admin commands are intentionally rejected until a safe actor-verification flow is implemented.`
