
/*
window.addEventListener("load",	() => {

	if (!("Notification" in window)) {
		console.log("this browser does not support notif");
		return;
	}

	Notification.requestPermission().then( (permission) => {
		console.log("permission: ", permission);
	});

	const button = document.querySelector("#buttonID");

	if (window.self !== window.top) {
		button.textContent = "voir résultat";
		button.addEventListener("click", () => {
			window.open(location.href);
		});
	}

	button.addEventListener("click", () => {
		var events;
		fetch("/evenements/index.json")
			.then((res) => {
				console.log("1");
				res.json()
			})
			.then((data) => {
				console.log("events: "+ data);
				events = data;
			})
			console.log("events2: "+ events);
		if (Notification?.permission === "granted") {
			let i = 0;
			if (events.length == 0) {
				return;
			}
			var start = Date.now();

			var nextEvent = Date.parse(events[0].startDate);

			var hourDuration = 60 * 60 * 1000;
			var dayDuration = 24 * hourDuration;

			var notif1weekBefore = 7 * dayDuration;
			var notif3DaysBefore = 3 * dayDuration;
			var notifTheDayBefore = 6 * hourDuration;

			var soon = 6 * dayDuration + 2 * hourDuration + 16 * 60 * 1000;

			var nextNotif = nextEvent - start - soon;
			console.log("next notif:"+ +nextNotif + " " +  new Date(nextNotif)+ " " + events[0].startDate);
			var timeout = setTimeout(() => {
				const n = new Notification(`bal ce soir ${i}`, {
					requireInteraction: true,
				});
			}, 2000);



		} else {
			alert("coucou");
		}
	});
});
*/
