package ontyranny

import (
	"goirc/internal/responder"
	"math/rand/v2"
)

type lesson struct {
	Number  int
	Text    string
	Youtube string
}

var lessons = []lesson{
	{
		Number:  1,
		Text:    "Do not obey in advance. Most of the power of authoritarianism is freely given. In times like these, individuals think ahead about what a more repressive government will want, and then offer themselves without being asked. A citizen who adapts in this way is teaching power what it can do.",
		Youtube: "https://www.youtube.com/watch?v=9tocssf3w80",
	},
	{
		Number:  2,
		Text:    "Defend institutions. It is institutions that help us to preserve decency. They need our help as well. Do not speak of “our institutions” unless you make them yours by acting on their behalf. Institutions do not protect themselves. So choose an institution you care about and take its side.",
		Youtube: "https://www.youtube.com/watch?v=BtS3M_paWhI",
	},
	{
		Number:  3,
		Text:    "Beware the one-party state. The parties that remade states and suppressed rivals were not omnipotent from the start. They exploited a historic moment to make political life impossible for their opponents. So support the multiparty system and defend the rules of democratic elections.",
		Youtube: "https://www.youtube.com/watch?v=sTtFTo4lJ14",
	},
	{
		Number:  4,
		Text:    "Take responsibility for the face of the world. The symbols of today enable the reality of tomorrow. Notice the swastikas and other signs of hate. Do not look away, and do not get used to them. Remove them yourself and set an example for others to do so.",
		Youtube: "https://www.youtube.com/watch?v=ysCCEuMC6xo",
	},
	{
		Number:  5,
		Text:    "Remember professional ethics. When political leaders set a negative example, professional commitments to just practice become important. It is hard to subvert a rule-of-law state without lawyers, or to hold show trials without judges. Authoritarians need obedient civil servants, and concentration camp directors seek businessmen interested in cheap labor.",
		Youtube: "https://www.youtube.com/watch?v=F75dhfkXjw8",
	},
	{
		Number:  6,
		Text:    "Be wary of paramilitaries. When the men with guns who have always claimed to be against the system start wearing uniforms and marching around with torches and pictures of a leader, the end is nigh. When the pro-leader paramilitary and the official police and military intermingle, the end has come.",
		Youtube: "https://www.youtube.com/watch?v=F75dhfkXjw8",
	},
	{
		Number:  7,
		Text:    "Be reflective if you must be armed. If you carry a weapon in public service, God bless you and keep you. But know that evils of the past involved policemen and soldiers finding themselves, one day, doing irregular things. Be ready to say no.",
		Youtube: "https://www.youtube.com/watch?v=-jNOevQIboY",
	},
	{
		Number:  8,
		Text:    "Stand out. Someone has to. It is easy to follow along. It can feel strange to do or say something different. But without that unease, there is no freedom. Remember Rosa Parks. The moment you set an example, the spell of the status quo is broken, and others will follow.",
		Youtube: "https://www.youtube.com/watch?v=oxIT54T7N_Y",
	},
	{
		Number:  9,
		Text:    "Be kind to our language. Avoid pronouncing the phrases everyone else does. Think up your own way of speaking, even if only to convey that thing you think everyone is saying. Make an effort to separate yourself from the Internet. Read books.",
		Youtube: "https://www.youtube.com/watch?v=xi-XQrdpG_o",
	},
	{
		Number:  10,
		Text:    "Believe in truth. To abandon facts is to abandon freedom. If nothing is true, then no one can criticize power because there is no basis upon which to do so. If nothing is true, then all is spectacle. The biggest wallet pays for the most blinding lights.",
		Youtube: "https://www.youtube.com/watch?v=FdHkkfB_7X0",
	},
	{
		Number:  11,
		Text:    "Investigate. Figure things out for yourself. Spend more time with long articles. Subsidize investigative journalism by subscribing to print media. Realize that some of what is on the Internet is there to harm you. Learn about sites that investigate propaganda campaigns (some of which come from abroad). Take responsibility for what you communicate to others.",
		Youtube: "https://www.youtube.com/watch?v=DLFv-d8AleQ",
	},
	{
		Number:  12,
		Text:    "Make eye contact and small talk. This is not just polite. It is part of being a citizen and a responsible member of society. It is also a way to stay in touch with your surroundings, break down social barriers, and understand whom you should and should not trust. If we enter a culture of denunciation, you will want to know the psychological landscape of your daily life.",
		Youtube: "https://www.youtube.com/watch?v=j2Ol6ZVDQPU",
	},
	{
		Number:  13,
		Text:    "Practice corporeal politics. Power wants your body softening in your chair and your emotions dissipating on the screen. Get outside. Put your body in unfamiliar places with unfamiliar people. Make new friends and march with them.",
		Youtube: "https://www.youtube.com/watch?v=laRIf1QXYek",
	},
	{
		Number:  14,
		Text:    "Establish a private life. Nastier rulers will use what they know about you to push you around. Scrub your computer of malware. Remember that email is skywriting. Consider using alternative forms of the Internet, or simply using it less. Have personal exchanges in person. For the same reason, resolve any legal trouble.",
		Youtube: "https://www.youtube.com/watch?v=Rf-T1vDz63g",
	},
	{
		Number:  15,
		Text:    "Contribute to good causes. Be active in organizations, political or not, that express your own view of life. Pick a charity or two and set up autopay.",
		Youtube: "https://www.youtube.com/watch?v=hEUF5IHuXjI",
	},
	{
		Number:  16,
		Text:    "Learn from peers in other countries. Keep up your friendships abroad, or make new friends abroad. The present difficulties in the United States are an element of a larger trend. And no country is going to find a solution by itself. Make sure you and your family have passports.",
		Youtube: "https://www.youtube.com/watch?v=CLJihII6P6k",
	},
	{
		Number:  17,
		Text:    "Listen for dangerous words. Be alert to the use of the words extremism and terrorism. Be alive to the fatal notions of emergency and exception. Be angry about the treacherous use of patriotic vocabulary.",
		Youtube: "https://www.youtube.com/watch?v=znVmAeYV1vM",
	},
	{
		Number:  18,
		Text:    "Be calm when the unthinkable arrives. Modern tyranny is terror management. When the terrorist attack comes, remember that authoritarians exploit such events in order to consolidate power. Do not fall for it.",
		Youtube: "https://www.youtube.com/watch?v=mQ0Ds9K4mi8",
	},
	{
		Number:  19,
		Text:    "Be a patriot. Set a good example of what America means for the generations to come.",
		Youtube: "https://www.youtube.com/watch?v=LEYD9W8Atmw",
	},
	{
		Number:  20,
		Text:    "Be as courageous as you can. If none of us is prepared to die for freedom, then all of us will die under tyranny.",
		Youtube: "https://www.youtube.com/watch?v=AB0vux9v5s0",
	},
}

func Handle(params responder.Responder) error {
	lesson := lessons[rand.IntN(len(lessons))]

	params.Privmsgf(params.Target(), "%s: Lesson %d: %s %s", params.Target, lesson.Number, lesson.Text, lesson.Youtube)

	return nil
}
