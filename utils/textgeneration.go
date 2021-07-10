// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package utils

import (
	"math/rand"
	"strings"
)

const (
	ALPHANUMERIC = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ01234567890"
	LOWERCASE    = "abcdefghijklmnopqrstuvwxyz"
)

// FuzzyStringsPosts are strings that should pass as acceptable posts
var FuzzyStringsPosts = []string{
	`**[1] - [Markdown Tests]**
_italics_
more _italics_
**bold**
more **bold**
**_bold-italic_**
more **_bold-italic_*8
~~strikethrough~~
more ~~strikethrough~~
` + "```" + `
multi-line code block<enter here>
multi-line code block
emoji that should not render in code block: :ice_cream:
` + "```" + `
` + "`monospace`" + `
[Link to Mattermost](www.mattermost.com)
Inline Image with link, alt text, and hover text: ![Build Status](https://travis-ci.org/mattermost/mattermost-server.svg?branch=master)](https://travis-ci.org/mattermost/mattermost-server)

Three types of lines:
***
___
---
`,

	` **[2] - **[More Markdown Tests]**
> i am a blockquote!

> i am a 2nd multiline 
> quote.
i am text right after a multiline quote, but not in the quote

* list item
* another list item
  * indented list item

1. numbered list, item number 1
2. item number two

`,

	` **[3]** - **[More Markdown Tests]**

Table

| Left-Aligned  | Center Aligned  | Right Aligned |
| :------------ |:---------------:| -----:|
| Left column 1 | this text       |  $100 |
| Left column 2 | is              |   $10 |
| Left column 3 | centered        |    $1 |

Ugly table

Markdown | Less | Pretty
--- | --- | ---
*Still* | ~~renders~~ | **nicely**
1 | 2 | 3

# Large heading
## Smaller heading
### Even smaller heading
# Large heading
## Smaller heading
### Even smaller heading

`,

	` **[4]** - **[More Markdown Tests]**
# This is a heading

I am a multiline
text.

#### I am a level four heading

` + "```tex" + `
f(x) = \int_{-\infty}^\infty
    \hat f(\xi)\,e^{2 \pi i \xi x}
    \,d\xi
` + "```" + `
* This was some tex code*
`,

	`**[5]** - **[Markdown and automatic preview of content test]**

## This should display a preview for the given vine url

Some text *before* the link
And a smiley :)
https://vine.co/v/eDeVgbFrt9L

Some more text here
and here
and even more here
`,

	`**[6]** - **[More markdown and automatic preview of content test]**

## Only the first given url should render an "attachment"

Lets also add a table here, because why not
| Left-Aligned  | Center Aligned  | Right Aligned |
| :------------ |:---------------:| -----:|
| Left column 1 | this text       |  $100 |
| Left column 2 | is              |   $10 |
| Left column 3 | centered        |    $1 |

Wiki should render:
http://en.wikipedia.org/wiki/Foo
https://vine.co/v/eDeVgbFrt9L
`,

	`**[7] [Image Test]**

## this *should* display an image

http://37.media.tumblr.com/tumblr_mavsumGGAd1qboaw8o1_500.jpg
`,

	/*	`**[2] [Username Linking Test]**
		I saw @alice--and I said "Hi @alice!" then "What's up @alice?" and then @alice, was totally @alice; she just "@alice"'d me and walked on by. That's @alice...
		@alice‽‽
		`,

			`**[3] [Mention Highlighting Test]**
		`,*/

	`**[8] [Emoji Display Test 1]**
:+1: :-1: :100: :1234: :8ball: :a: :ab: :abc: :abcd: :accept: 
:aerial_tramway: :airplane: :alarm_clock: :ambulance: :anchor: :angel: :anger: :angry: :anguished: :ant: 
:apple: :aquarius: :aries: :arrow_backward: :arrow_double_down: :arrow_double_up: :arrow_down: :arrow_down_small: :arrow_forward: :arrow_heading_down:
:arrow_heading_up: :arrow_left: :arrow_lower_left: :arrow_lower_right: :arrow_right: :arrow_right_hook: :arrow_up: :arrow_up_down:
:arrow_upper_left: :arrow_upper_right: :arrows_clockwise: :arrows_counterclockwise: :art: :articulated_lorry: :astonished: :atm:  :arrow_up_small: :b:
:baby: :baby_bottle: :baby_chick: :baby_symbol: :back: :baggage_claim: :balloon: :ballot_box_with_check: :bamboo: :banana:
:bangbang: :bank: :bar_chart: :barber: :baseball: :basketball: :bath: :bathtub: :battery: :bear:
:bee: :beer: :beers: :beetle: :beginner: :bell: :bento: :bicyclist: :bike: :bikini: 
:bird: :birthday: :black_circle: :black_joker: :black_medium_small_square: :black_medium_square: :black_nib: :black_small_square: :black_square: :black_square_button:
:blossom: :blowfish: :blue_book: :blue_car: :blue_heart: :blush: :boar: :boat: :bomb: :book:
:bookmark: :bookmark_tabs: :books: :boom: :boot: :bouquet: :bow: :bowling: :bowtie: :boy:
:bread: :bride_with_veil: :bridge_at_night: :briefcase: :broken_heart: :bug: :bulb: :bullettrain_front: :bullettrain_side: :bus: 
:busstop: :bust_in_silhouette: :busts_in_silhouette: :cactus: :cake: :calendar: :calling: :camel: :camera: :cancer: 
:candy: :capital_abcd: :capricorn: :car: :card_index: :carousel_horse: :cat: :cat2: :cd: :chart: 
:chart_with_downwards_trend: :chart_with_upwards_trend: :checkered_flag: :cherries: :cherry_blossom: :chestnut: :chicken: :children_crossing: :chocolate_bar: :christmas_tree:
:church: :cinema: :circus_tent: :city_sunrise: :city_sunset: :cl: :clap: :clapper: :clipboard: :clock1: 
:clock10: :clock1030: :clock11: :clock1130: :clock12: :clock1230: :clock130: :clock2: :clock230: :clock3: 
:clock330: :clock4: :clock430: :clock5: :clock530: :clock6: :clock630: :clock7: :clock730: :clock8: 
:clock830: :clock9: :clock930: :closed_book: :closed_lock_with_key: :closed_umbrella: :cloud: :clubs: :cn: :cocktail:
:coffee: :cold_sweat: :collision: :computer: :confetti_ball: :confounded: :confused: :congratulations: :construction: :construction_worker:
:convenience_store: :cookie: :cool: :cop: :copyright: :corn: :couple: :couple_with_heart: :couplekiss: :cow:
:cow2: :credit_card: :crescent_moon: :crocodile: :crossed_flags: :crown: :cry: :crying_cat_face: :crystal_ball: :cupid: 
:curly_loop: :currency_exchange: :curry: :custard: :customs: :cyclone: :dancer: :dancers: :dango: :dart:
:dash: :date: :de: :deciduous_tree: :department_store: :diamond_shape_with_a_dot_inside: :diamonds: :disappointed: :disappointed_relieved: :dizzy:
:dizzy_face: :do_not_litter: :dog: :dog2: :dollar: :dolls: :dolphin: :donut: :door: :doughnut:
:dragon: :dragon_face: :dress: :dromedary_camel: :droplet: :dvd: :e-mail: :ear: :ear_of_rice: :earth_africa: 
:earth_americas: :earth_asia: :egg: :eggplant: :eight: :eight_pointed_black_star: :eight_spoked_asterisk: :electric_plug: :elephant: :email: 
 :end: :envelope: :es: :euro: :european_castle: :european_post_office: :evergreen_tree: :exclamation: :expressionless: :eyeglasses: 
:eyes: :facepunch: :factory: :fallen_leaf: :family: :fast_forward: :fax: :fearful: :feelsgood: :feet: 
:ferris_wheel: :file_folder: :finnadie: :fire: :fire_engine: :fireworks: :first_quarter_moon: :first_quarter_moon_with_face: :fish: :fish_cake:
:fishing_pole_and_fish: :fist: :five: :flags: :flashlight: :floppy_disk: :flower_playing_cards: :flushed: :foggy: :football:
:fork_and_knife: :fountain: :four: :four_leaf_clover: :fr: :free: :fried_shrimp: :fries: :frog: :frowning:
:fu: :fuelpump: :full_moon: :full_moon_with_face: :game_die: :gb: :gem: :gemini: :ghost: :gift:`,

	`**[9] [Emoji Display Test 2]**
:gift_heart: :girl: :globe_with_meridians: :goat: :goberserk: :godmode: :golf: :grapes: :green_apple: :green_book:
:green_heart: :grey_exclamation: :grey_question: :grimacing: :grin: :grinning: :guardsman: :guitar: :gun: :haircut: 
:hamburger: :hammer: :hamster: :hand: :handbag: :hankey: :hash: :hatched_chick: :hatching_chick: :headphones:
:hear_no_evil: :heart: :heart_decoration: :heart_eyes: :heart_eyes_cat: :heartbeat: :heartpulse: :hearts: :heavy_check_mark: :heavy_division_sign:
:heavy_dollar_sign: :heavy_exclamation_mark: :heavy_minus_sign: :heavy_multiplication_x: :heavy_plus_sign: :helicopter: :herb: :hibiscus: :high_brightness: :high_heel:
:hocho: :honey_pot: :honeybee: :horse: :horse_racing: :hospital: :hotel: :hotsprings: :hourglass: :hourglass_flowing_sand:
:house: :house_with_garden: :hurtrealbad: :hushed: :ice_cream: :icecream: :id: :ideograph_advantage: :imp: :inbox_tray: 
:incoming_envelope: :information_desk_person: :information_source: :innocent: :interrobang: :iphone: :it: :izakaya_lantern: :jack_o_lantern:
:japan: :japanese_castle: :japanese_goblin: :japanese_ogre: :jeans: :joy: :joy_cat: :jp: :key: :keycap_ten:
:kimono: :kiss: :kissing: :kissing_cat: :kissing_closed_eyes: :kissing_face: :kissing_heart: :kissing_smiling_eyes: :koala: :koko:
:kr: :large_blue_circle: :large_blue_diamond: :large_orange_diamond: :last_quarter_moon: :last_quarter_moon_with_face: :laughing: :leaves: :ledger: :left_luggage:
:left_right_arrow: :leftwards_arrow_with_hook: :lemon: :leo: :leopard: :libra: :light_rail: :link: :lips: :lipstick:
:lock: :lock_with_ink_pen: :lollipop: :loop: :loudspeaker: :love_hotel: :love_letter: :low_brightness: :m: :mag:
:mag_right: :mahjong: :mailbox: :mailbox_closed: :mailbox_with_mail: :mailbox_with_no_mail: :man: :man_with_gua_pi_mao: :man_with_turban: :mans_shoe:
:maple_leaf: :mask: :massage: :meat_on_bone: :mega: :melon: :memo: :mens: :metal: :metro:
:microphone: :microscope: :milky_way: :minibus: :minidisc: :mobile_phone_off: :money_with_wings: :moneybag: :monkey: :monkey_face: 
:monorail: :mortar_board: :mount_fuji: :mountain_bicyclist: :mountain_cableway: :mountain_railway: :mouse: :mouse2: :movie_camera: :moyai:
:muscle: :mushroom: :musical_keyboard: :musical_note: :musical_score: :mute: :nail_care: :name_badge: :neckbeard: :necktie:
:negative_squared_cross_mark: :neutral_face: :new: :new_moon: :new_moon_with_face: :newspaper: :ng: :nine: :no_bell:
:no_bicycles: :no_entry: :no_entry_sign: :no_good: :no_mobile_phones: :no_mouth: :no_pedestrians: :no_smoking: :non-potable_water: :nose:
:notebook: :notebook_with_decorative_cover: :notes: :nut_and_bolt: :o: :o2: :ocean: :octocat: :octopus: :oden: 
:office: :ok: :ok_hand: :ok_woman: :older_man: :older_woman: :on: :oncoming_automobile: :oncoming_bus: :oncoming_police_car:
:oncoming_taxi: :one: :open_file_folder: :open_hands: :open_mouth: :ophiuchus: :orange_book: :outbox_tray: :ox: :package:
:page_facing_up: :page_with_curl: :pager: :palm_tree: :panda_face: :paperclip: :parking: :part_alternation_mark: :partly_sunny: :passport_control:
:paw_prints: :peach: :pear: :pencil: :pencil2: :penguin: :pensive: :performing_arts: :persevere: :person_frowning:
:person_with_blond_hair: :person_with_pouting_face: :phone: :pig: :pig2: :pig_nose: :pill: :pineapple: :pisces: :pizza:
`,

	`**[10] [Emoji Display Test 3]**
:plus1: :point_down: :point_left: :point_right: :point_up: :point_up_2: :police_car: :poodle: :poop: :post_office:
:postal_horn: :postbox: :potable_water: :pouch: :poultry_leg: :pound: :pouting_cat: :pray: :princess: :punch: 
:purple_heart: :purse: :pushpin: :put_litter_in_its_place: :question: :rabbit: :rabbit2: :racehorse: :radio: :radio_button:
:rage: :rage1: :rage2: :rage3: :rage4: :railway_car: :rainbow: :raised_hand: :raised_hands: :raising_hand:
:ram: :ramen: :rat: :recycle: :red_car: :red_circle: :registered: :relaxed: :relieved: :repeat: 
:repeat_one: :restroom: :revolving_hearts: :rewind: :ribbon: :rice: :rice_ball: :rice_cracker: :rice_scene: :ring: 
:rocket: :roller_coaster: :rooster: :rose: :rotating_light: :round_pushpin: :rowboat: :ru:
:rugby_football: :runner: :running: :running_shirt_with_sash: :sa: :sagittarius: :sailboat: :sake: :sandal: :santa: 
:satellite: :satisfied: :saxophone: :school: :school_satchel: :scissors: :scorpius: :scream: :scream_cat: :scroll:
:seat: :secret: :see_no_evil: :seedling: :seven: :shaved_ice: :sheep: :shell: :ship: :shipit:
:shirt: :shit: :shoe: :shower: :signal_strength: :six: :six_pointed_star: :ski: :skull: :sleeping:
:sleepy: :slot_machine: :small_blue_diamond: :small_orange_diamond: :small_red_triangle: :small_red_triangle_down: :smile: :smile_cat: :smiley: :smiley_cat:
:smiling_imp: :smirk: :smirk_cat: :smoking: :snail: :snake: :snowboarder: :snowflake: :snowman: :sob:
:soccer: :soon: :sos: :sound: :space_invader: :spades: :spaghetti: :sparkle: :sparkler: :sparkles:
:sparkling_heart: :speak_no_evil: :speaker: :speech_balloon: :speedboat: :squirrel: :star: :star2: :stars: :station:
:statue_of_liberty: :steam_locomotive: :stew: :straight_ruler: :strawberry: :stuck_out_tongue: :stuck_out_tongue_closed_eyes: :stuck_out_tongue_winking_eye: :sun_with_face: :sunflower:
 :sunglasses: :sunny: :sunrise: :sunrise_over_mountains: :surfer: :sushi: :suspect: :suspension_railway: :sweat: :sweat_drops:
:sweat_smile: :sweet_potato: :swimmer: :symbols: :syringe: :tada: :tanabata_tree: :tangerine: :taurus: :taxi:
:tea: :telephone: :telephone_receiver: :telescope: :tennis: :tent: :thought_balloon: :three: :thumbsdown: :thumbsup: 
:ticket: :tiger: :tiger2: :tired_face: :tm: :toilet: :tokyo_tower: :tomato: :tongue: :top:
:tophat: :tractor: :traffic_light: :train: :train2: :tram: :triangular_flag_on_post: :triangular_ruler: :trident: :triumph:
:trolleybus: :trollface: :trophy: :tropical_drink: :tropical_fish: :truck: :trumpet: :tshirt: :tulip: :turtle: 
:tv: :twisted_rightwards_arrows: :two: :two_hearts: :two_men_holding_hands: :two_women_holding_hands: 
:uk: :umbrella: :unamused: :underage: :unlock: :up: :us: :v: :vertical_traffic_light: :vhs: 
:vibration_mode: :video_camera: :video_game: :violin: :virgo: :volcano: :vs: :walking: :waning_crescent_moon: :waning_gibbous_moon:
:warning: :watch: :water_buffalo: :watermelon: :wave: :wavy_dash: :waxing_crescent_moon: :waxing_gibbous_moon: :wc: :weary:
:wedding: :whale: :whale2: :wheelchair: :white_check_mark: :white_circle: :white_flower: :white_large_square: :white_medium_small_square:  :white_medium_square:
:white_small_square: :white_square_button: :wind_chime: :wine_glass: :wink: :wolf: :woman: :womans_clothes: :womans_hat: :womens:
:worried: :wrench: :x: :yellow_heart: :yen: :yum: :zap: :zero: :zzz:
Unnamed: :u5272: :u5408: :u55b6: :u6307: :u6708: :u6709: :u6e80: :u7121: :u7533: :u7981: :u7a7a:
`,

	`**[11] [Auto Linking]**
#### should be turned into links:
http://example.com
https://example.com
www.example.com
www.example.com/index
www.example.com/index.html
www.example.com/index/sub
www.example.com/index?params=1
www.example.com/index?params=1&other=2
www.example.com/index?params=1;other=2
http://example.com:8065
<http://example.com>
<www.example.com>
http://www.example.com/_/page
www.example.com/_/page
https://en.wikipedia.org/wiki/🐬
https://en.wikipedia.org/wiki/Rendering_(computer_graphics)
http://127.0.0.1
http://192.168.1.1:4040
http://[::1]:80
http://[::1]:8065
https://[::1]:80
http://[2001:0:5ef5:79fb:303a:62d5:3312:ff42]:80
http://[2001:0:5ef5:79fb:303a:62d5:3312:ff42]:8065
https://[2001:0:5ef5:79fb:303a:62d5:3312:ff42]:443
http://username:password@example.com
http://username:password@127.0.0.1
http://username:password@[2001:0:5ef5:79fb:303a:62d5:3312:ff42]:80
test@example.com

#### should be turned into links which link to the correct place:
[example link](example.com) links to ` + "`" + `http://example.com` + "`" + `
[example.com](example.com) links to ` + "`" + `http://example.com` + "`" + `
[example.com/other](example.com) links to ` + "`" + `http://example.com` + "`" + `
[example.com/other_link](example.com/example) links to ` + "`" + `http://example.com/example` + "`" + `
www.example.com links to ` + "`" + `http://www.example.com` + "`" + `
https://example.com links to ` + "`" + `https://example.com` + "`" + `and not ` + "`" + `http://example.com` + "`" + `
https://en.wikipedia.org/wiki/🐬 links to the Wikipedia article on dolphins
https://en.wikipedia.org/wiki/URLs#Syntax links to the Syntax section of the Wikipedia article on URLs
test@example.com links to ` + "`" + `mailto:test@example.com` + "`" + `
[email link](mailto:test@example.com) links to ` + "`" + `mailto:test@example.com` + "`" + `and not ` + "`" + `http://mailto:test@example.com` + "`" + `
[other link](ts3server://example.com) links to ` + "`" + `ts3server://example.com` + "`" + `and not ` + "`" + `http://ts3server://example.com` + "`" + `

#### should not be turned into links:
example.com
readme.md
<example.com>
http://
@example.com

#### should only turn the actual link into a link and not change surrounding text
(http://example.com)
(test@example.com)
This is a sentence with a http://example.com in it.
This is a sentence with a [link](http://example.com) in it.
This is a sentence with a http://example.com/_/underscore in it.
This is a sentence with a link (http://example.com) in it.
This is a sentence with a (https://en.wikipedia.org/wiki/Rendering_(computer_graphics)) in it.
This is a sentence with a http://192.168.1.1:4040 in it.
This is a sentence with a https://::1 in it.
This is a link to http://example.com.
`,

	"*", "?", ".", "}{][)(><", "{}[]()<>",

	"qahwah ( قهوة)",
	"שָׁלוֹם עֲלֵיכֶם",
	"Ramen チャーシュー chāshū",
	"言而无信",
	"Ṫ͌ó̍ ̍͂̓̍̍̀i̊ͯ͒",
	"&amp; &lt; &qu",

	"' or '1'='1' -- ",
	"' or '1'='1' ({ ",
	"' or '1'='1' /* ",
	"1;DROP TABLE users",

	"<b><i><u><strong><em>",

	"sue@thatmightbe",
	"sue@thatmightbe.",
	"sue@thatmightbe.c",
	"sue@thatmightbe.co",
	"su+san@thatmightbe.com",
	"a@b.中国",
	"1@2.am",
	"a@b.co.uk",
	"a@b.cancerresearch",
	"local@[127.0.0.1]",

	"!@$%^&:*.,/|;'\"+=?`~#",
	"'\"/\\\"\"''\\/",
	"gggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggg",
	"gggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggg",
	"ą ć ę ł ń ó ś ź ż č ď ě ň ř š ť ž ă î ø å æ á é í ó ú Ç Ğ İ Ö Ş Ü",
	"abcdefghijklmnopqrstuvwrxyz0123456789 -_",
	"Ṫ͌ó̍ ̍͂̓̍̍̀i̊ͯ͒nͧ̍̓̃͋vok̂̓ͤ̓̂ěͬ ͆tͬ̐́̐͆h̒̏͌̓e͂ ̎̊h̽͆ͯ̄ͮi͊̂ͧͫ̇̃vͥͦ́ẻͤ-͒m̈́̀i̓ͮ͗̑͌̆̅n̓̓ͨd̊̑͛̔̚ ͨͮ̊̾rͪeͭͭ͑ͧ́͋p̈́̅̚rͧe̒̈̌s̍̽ͩ̓̇e͗n̏͊ͬͭtͨ͆ͤ̚iͪ͗̍n͐͒g̾ͦ̎ ͥ͌̽̊ͩͥ͗c̀ͬͣha̍̏̉ͪ̈̚o̊̏s̊̋̀̏̽̚.͒ͫ͛͛̎ͥ",
	"H҉̵̞̟̠̖̗̘Ȅ̐̑̒̚̕̚ IS C̒̓̔̿̿̿̕̚̚̕̚̕̚̕̚̕̚̕̚OMI҉̵̞̟̠̖̗̘NG > ͡҉҉ ̵̡̢̛̗̘̙̜̝̞̟̠͇̊̋̌̍̎̏̿̿̿̚ ҉ ҉҉̡̢̡̢̛̛̖̗̘̙̜̝̞̟̠̖̗̘̙̜̝̞̟̠̊̋̌̍̎̏̐̑̒̓̔̊̋̌̍̎̏̐̑ ͡҉҉",

	"<a href=\"//www.google.com\">Teh Googles</a>",
	"<img src=\"//upload.wikimedia.org/wikipedia/meta/b/be/Wikipedia-logo-v2_2x.png\" />",
	"&amp; &lt; &quot; &apos;",
	" %21 %23 %24 %26 %27 %28 %29 %2A	%2B	%2C	%2F	%3A	%3B	%3D	%3F	%40	%5B	%5D %0D %0A %0D%0A %20 %22 %25 %2D %2E %3C %3E %5C %5E %5F %60 %7B %7C %7D %7E",

	";alert('Well this is awkward.');",
	"<script type='text/javascript'>alert('yay puppies');</script>",

	"http?q=foobar%0d%0aContent-\nLength:%200%0d%0a%0d%0aHTTP/1.1%20200%20OK%0d%0aContent-\nType:%20text/html%0d%0aContent-Length:%2019%0d%0a%0d%0a<html>Shazam</html>",

	"apos'trophe@thatmightbe.com",
	"apos''''trophe@thatmightbe.com",
	"su+s+an@thatmightbe.com",
	"per.iod@thatmightbe.com",
	"per..iods@thatmightbe.com",
	".period@thatmightbe.com",
	"tom(comment)@thatmightbe.com",
	"(comment)tom@thatmightbe.com",
	"\"quotes\"@thatmightbe.com",
	"\"\\\"(),:;<>@[\\]\"@thatmightbe.com",
	"a!#$%&'*+-/=?^_`{|}~b@thatmightbe.com",
	"jill@(comment)example.com",
	"jill@example.com(comment)",
	"ben@ggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggg.com",
	"judy@gggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggg.com",
	"ggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggg@AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA.com",
}

// FuzzyStringsNames are strings that should pass as acceptable team names
var FuzzyStringsNames = []string{
	"*",
	"?",
	".",
	"}{][)(><",
	"{}[]()<>",

	"qahwah ( قهوة)",
	"שָׁלוֹם עֲלֵיכֶם",
	"Ramen チャーシュー chāshū",
	"言而无信",
	"Ṫ͌ó̍ ̍͂̓̍̍̀i̊ͯ͒",
	"&amp; &lt; &qu",

	"' or '1'='1' -- ",
	"' or '1'='1' ({ ",
	"' or '1'='1' /* ",
	"1;DROP TABLE users",

	"<b><i><u><strong><em>",

	"sue@thatmightbe",
	"sue@thatmightbe.",
	"sue@thatmightbe.c",
	"sue@thatmightbe.co",
	"sue @ thatmightbe.com",
	"apos'trophe@thatmightbe.com",
	"apos''''trophe@thatmightbe.com",
	"su+san@thatmightbe.com",
	"su+s+an@thatmightbe.com",
	"per.iod@thatmightbe.com",
	"per..iods@thatmightbe.com",
	".period@thatmightbe.com",
	"tom(comment)@thatmightbe.com",
	"(comment)tom@thatmightbe.com",
	"\"quotes\"@thatmightbe.com",
	"\"\\\"(),:;<>@[\\]\"@thatmightbe.com",
	"a!#$%&'*+-/=?^_`{|}~b@thatmightbe.com",
	"local@[127.0.0.1]",
	"jill@(comment)example.com",
	"jill@example.com(comment)",
	"a@b.中国",
	"1@2.am",
	"a@b.co.uk",
	"a@b.cancerresearch",

	"<a href=\"//www.google.com\">Teh Googles</a>",
	"<img src=\"//upload.wikimedia.org/wikipelogo-v2_2x.png\" />",
	"<b><i><u><strong><em>",
	"&amp; &lt; &quot; &apos;",

	";alert('Well this is awkward.');",
	"<script type='text/javascript'>alert('yay puppies');</script>",

	"Ṫ͌ó̍ ̍͂̓̍̍̀i̊ͯ͒nͧ̍̓̃͋v",
	"H҉̵̞̟̠̖̗̘Ȅ̐̐̑̒̚OMI҉̵̞̟̠",
}

// FuzzyStringsEmails are strings that should pass as acceptable emails
var FuzzyStringsEmails = []string{
	"sue@thatmightbe",
	"sue@thatmightbe.c",
	"sue@thatmightbe.co",
	"su+san@thatmightbe.com",
	"1@2.am",
	"a@b.co.uk",
	"a@b.cancerresearch",
	"su+s+an@thatmightbe.com",
	"per.iod@thatmightbe.com",
}

// Lovely giberish for all to use
const GibberishText = `
Thus one besides much goodness shyly far some hyena overtook since rhinoceros nodded withdrew wombat before deserved apart a alongside the far dalmatian less ouch where yet a salmon.
Then jeez far marginal hey aboard more as leaned much oversold that inside spoke showed much went crud close save so and and after and informally much lion commendably less conductive oh excepting conductive compassionate jeepers hey a much leopard alas woolly untruthful outside snug rashly one cunning past fabulous adjusted far woodchuck and and indecisive crud loving exotic less resolute ladybug sprang drank under following far the as hence passably stolidly jeez the inset spaciously more cozily fishily the hey alas petted one audible yikes dear preparatory darn goldfinch gosh a then as moth more guinea.
Timid mislaid as salamander yikes alas ouch much that goldfinch shark in before instead dear one swore vivid versus one until regardless sang panther tolerable much preparatory hardily shuddered where coquettish far sheep coarsely exaggerated preparatory because cordial awesome gradually nutria that dear mocking behind off staunchly regarding a the komodo crud shrewd well jeez iguanodon strove strived and moodily and sought and and mounted gosh aboard crud spitefully boa.
One as highhanded fortuitous angelfish so one woodchuck dazedly kangaroo nasty instead far parrot away the worm yet testy where caribou a cuckoo onto dear reined because less tranquil kindhearted and shuddered plankton astride monkey methodically above evasive otter this wrung and courageous iguana wayward along cowered prior a.
Freely since ouch octopus the heated apart on hey the some pending placed fearless jeepers hardheadedly more that less jolly bit cuddled.
Caterpillar laboriously far wistful spilled aside far oriole newt and immeasurably yikes revealed raptly obdurately definitely scallop titilatingly one alongside monumentally ouch much wretched the spoke a before alas insolent abortive that turned hey hare much poignantly re-laid goodness yet the dear compassionate a hey scooped sped darn warmly oh and more darn craven that overtook fell and bluebird misheard that needless less ravenously in positively far romantically some babbled that rose honey then immaturely this and jollily irresistible much rarely earthworm parrot wow.
Less less bluntly jeez at goodness panther opposite oh purred a pathetically mildly less cat badly much much on from obscure in gull off manatee hatchet goodness euphemistically hence or understandable after this so that thus shook hence that mindfully yellow behind far bat wayward thanks more wrote so the flapped however alas and mallard that temperately irritably yikes squirrel.
Some reset some therefore demonstrably considering dachshund kindhearted far wow far whispered far clung this by partook much upon fit inscrutably so affirmative diligently far grinned and manifestly hummingbird hello caudal considering when aboard much buoyantly that unfitting far attractively far during much crud baneful jeez one toneless cynically oh spurious athletic meadowlark much generously one subconsciously arguable much forthrightly hawk inoffensively.
Snorted tidy stiffly against one fiendishly began burst hey revealed a beside the soothingly ceremonially affirmatively cowered when fitted this static hello emoted assenting however while far that gross besides because and dear.
Far therefore the blushed momentously the however one a wholeheartedly and considering incessantly that neurotically wore firefly grouped impotently dear one abjectly goodness so far a honey far insolently far so greyhound between above raucously echidna more halfhearted thankful squid one.
Raccoon cockatoo this while but this a far among ouch and hey alas scallop black sane as yikes hello sexy far tacky and balked wrongly more near shrewdly the yet gosh much caribou ruthlessly a on far a threw well less at the one after.
Spoke touched barbarously before much thus therefore darn scratched oh howled the less much hello after and jeez flagrantly weirdly crud komodo fabulous the much some cow jeering much egregiously a bucolically a admirably jeepers essential when ouch and tapir this while and wolverine.
Cm more much in this rewrote ouch on from aside wildebeest crane saddled where much opposite endearingly hummingbird together some beside a the goodness dear ouch ouch struck the input smooched shrugged until slick as waked hawk sincere irksomely.
Camel the pulled this richly grimaced leopard more false thought dear militant added yikes supp infallibly set orca beat hello while accurately reliably while lorikeet one strategic less hello without and smooched across plankton but jeepers pangolin the rich seal sneered pre-set lynx on radical nasty alas onto more hence flabby outbid murkily congenially dived much lubber added far eccentrically turtle before outsold onto ouch thus much and hawk tolerable much knitted yikes shot much limpet one this woolly much however hence up angry up well.
Unicorn yawned hello boundless this when express jaded closed wept tranquil after came airily merry much dismounted for much extensively less interminably far one far armadillo pled dolphin alas nutria and more oh positively koala grizzly after falcon goat strict hooted next browbeat split more far far antagonistic lingering the depending pending sheared since up before jeepers distant mastodon dropped as this more some much set far infinitesimal well shark grasshopper as hey one via some fishy and immaturely remote where weasel leopard annoying correctly wherever that sniffled much mandrill on jeez adventurous much.
Jeepers before spitefully buoyant concentric the reset moth a darn decidedly baboon giraffe outrageously groundhog on one at more overslept gosh worm away far far less much hysteric showed on so rattlesnake the and immature yikes baneful hence wow lynx hence past scornfully groaned pounded dived this one outside dachshund scowled one prior tenable therefore before scratched much much drank hey while added rabbit shark and supp cut this ironic limpet hedgehog bound more rebuking the jeepers thorough while more far due but yikes nastily brave dangerous opened tangibly aside after acrimoniously one cackled scratched.
Canny salmon hatchet more far opposite much coughed excited expedient far lizard one indiscriminate yikes jeez powerlessly forcefully tiger rooster and brought far more during this sank onto after then less amorally rude unerring some alongside irrespective bat hungrily kangaroo extravagantly inside ouch much gosh dreadfully oh much darn prior as fired guinea.
Irksomely upon up for amicably one since contrary one until flamingo tarantula far koala despite easy well gazelle ungracefully rose less that under hey more criminal unique furrowed so disbanded normal where one a a hey circuitous ouch feverish for the kookaburra and pithy far far then more the versus cliquishly across oh and explicitly much therefore as tamely alongside underlay much yikes imminently off however far across instantaneous therefore wallaby evidently foul foretold as far a jeepers invidious bearish.
More and until scandalously after wallaby petted oh much as poked much caterpillar drank beside rode actively walking scooped weird this duteous that far before human during dear house thrust more flinched opposite that ahead in far.
The painful essential jeepers merrily proudly essential and less far dismounted inside mongoose beyond confessedly robin shined heron the during since according suggestively and less some strident combed alas much man-of-war forgave so and to then inanimately.
Beside far this this a crud polite cantankerous exclusively misheard pled far circuitously and frugal less more temperately gauche goldfinch oh against this along excitedly goodhearted more classically quit serenely outside vulture ouch after one a this yet.
Less and handsomely manatee some amidst much reined komodo busted exultingly but fatuously less across mighty goodness objective alas glaringly gregariously hello the since one pridefully much well placed far less goodness jellyfish unnecessary reciprocating a far stylistic gazed one.
Hey rethought excepting lamely much and naughtily amidst more since jeez then bluebird hence less bald by some brought left the across logic loyal brightly jeez capitally that less more forward rebound a yikes chose convulsively confidently repeated broadcast much dipped when awesomely or some some regal the scowled merry zebra since more credible so inescapably fetchingly and lantern that due dear one went gosh wow well furrowed much much specially spoiled as vitally instead the seriously some rooster irrespective well imprecisely rapidly more llama.
Up to and hey without pill that this squid alas brusque on inventoried and spread the more excepting aristocratically due piquant wove beneath that macaw in more until much grimaced far and jeez enticingly unicorn some far crab more barring purely jeepers clear groomed glaring hey dear hence before the this hello.`

func RandString(l int, charset string) string {
	ret := make([]byte, l)
	for i := 0; i < l; i++ {
		ret[i] = charset[rand.Intn(len(charset))]
	}
	return string(ret)
}

// func RandomEmail(length Range, charset string) string {
// 	emaillen := RandIntFromRange(length)
// 	username := RandString(emaillen, charset)
// 	domain := "simulator.amazonses.com"
// 	return "success+" + username + "@" + domain
// }

// func FuzzEmail() string {
// 	return FuzzyStringsEmails[RandIntFromRange(Range{0, len(FuzzyStringsEmails) - 1})]
// }

func RandomName(length Range, charset string) string {
	namelen := RandIntFromRange(length)
	return RandString(namelen, charset)
}

func FuzzName() string {
	return FuzzyStringsNames[RandIntFromRange(Range{0, len(FuzzyStringsNames) - 1})]
}

// Random selection of text for post
func RandomText(length Range, hashtags Range, mentions Range, users []string) string {
	textLength := RandIntFromRange(length)
	numHashtags := RandIntFromRange(hashtags)
	numMentions := RandIntFromRange(mentions)
	if textLength > len(GibberishText) || textLength < 0 {
		textLength = len(GibberishText)
	}
	startPosition := RandIntFromRange(Range{0, len(GibberishText) - textLength - 1})

	words := strings.Split(GibberishText[startPosition:startPosition+textLength], " ")
	for i := 0; i < numHashtags; i++ {
		randword := RandIntFromRange(Range{0, len(words) - 1})
		words = append(words, " #"+words[randword])
	}
	if len(users) > 0 {
		for i := 0; i < numMentions; i++ {
			randuser := RandIntFromRange(Range{0, len(users) - 1})
			words = append(words, " @"+users[randuser])
		}
	}

	// Shuffle the words
	for i := range words {
		j := rand.Intn(i + 1)
		words[i], words[j] = words[j], words[i]
	}

	return strings.Join(words, " ")
}

func FuzzPost() string {
	return FuzzyStringsPosts[RandIntFromRange(Range{0, len(FuzzyStringsPosts) - 1})]
}
