package photocycle

const (
	//StateErrZip represent photocycle state
	StateErrZip = -330 //Ошибка zip

	//StateErrEmptyFtp represent photocycle state
	StateErrEmptyFtp = -329 //Не загружен на FTP

	//-328	Ошибка проверки IM
	//-327	Ошибка проверки MD5
	//-326	Ошибка проверки
	//-325	Ошибка FTP
	//-323	Ошибка перепечатки
	//-322	Не верный статус
	//-321	Блокирован другим процессом

	//StateErrProductionNotSet represent photocycle state
	StateErrProductionNotSet = -320 //Не назначено производство

	//-319	Ошибка инициализации

	//StateErrStructureLoad represent photocycle state
	StateErrStructureLoad = -318 //Ошибка загрузки структуры

	//-317	Ошибка удаленной загрузки
	//-316	Ошибка удаленной подготовки

	//StateErrPreprocess represent photocycle state
	StateErrPreprocess = -315 //Ошибка подготовки

	//StateErrFileSystem represent photocycle state
	StateErrFileSystem = -314 //Ошибка файловой системы

	//StateErrWeb represent photocycle state
	StateErrWeb = -312 //Ошибка web

	//StateErrLoad represent photocycle state
	StateErrLoad = -311 //Ошибка загрузки

	//StateErrWrite represent photocycle state
	StateErrWrite = -310 //Ошибка записи.

	//StateErrRead represent photocycle state
	StateErrRead = -309 //Ошибка чтения.

	//-302	Hot folder лаболратории не найден
	//-301	Папка группы печати не найдена
	//-300	Ошибка размещения на печать
	//0	-

	//StateLoadWaite represent photocycle state
	StateLoadWaite = 100 //Ожидание загрузки

	//101	Загружать в первую очередь
	//102	Ожидание загрузки исправлен

	//StateCheckWeb represent photocycle state
	StateCheckWeb = 103 //Проверка web статуса

	//104	Web ok

	//StateLoadLock represent photocycle state
	StateLoadLock = 105 //Заблокирован для загрузки

	//107	Ожидание загрузки подзаказа

	//StateLoadStructure represent photocycle state
	StateLoadStructure = 108 //Загрузка структуры

	//109	Список файлов

	//StateLoad represent photocycle state
	StateLoad = 110 //Загрузка

	//StateUnzip represent photocycle state
	StateUnzip = 118 //Загрузка

	//114	Ожидание проверки
	//115	Проверка
	//120	Ошибка загрузки

	//StateLoadComplite represent photocycle state
	StateLoadComplite = 130 //Загрузка завершена

	//139	Ожидание цветокоррекции
	//140	Цветокоррекция
	//145	Ожидает перепечатки
	//146	Захвачен на перепечатку

	//StatePreprocessWaite represent photocycle state
	StatePreprocessWaite =150	//Ожидание подготовки

	//151	Подготовить в первую очередь
	//155	Проверка web статуса
	//156	Web ok
	//157	Заблокирован для подготовки
	//160	Ресайз
	//165	Подготовка книги

	//StatePreprocessIncomplite represent photocycle state
	StatePreprocessIncomplite = 170 //Ошибка подготовки

	//180	Подготовка завершена

	//StateConfirmation represent photocycle state
	StateConfirmation = 199 //Ожидание подтверждения заказа

	//StatePrintWaite represent photocycle state
	StatePrintWaite = 200 //Готов к печати

	//203	В очереди размещения на печать
	//205	Проверка web статуса
	//206	Web ok
	//209	Допечатная подготовка
	//210	Размещение на печать
	//215	Отмена размещения на печать
	//220	Автопечать

	//StatePrint represent photocycle state
	StatePrint = 250 //Размещен на печать

	//251	Перепечатка
	//255	Печатается
	//300	Напечатан
	//318	БиговкаФальцовка (б)
	//320	Фальцовка (б)
	//330	Ламинирование (о)
	//335	КрышкоДелка (о)
	//340	Листоподборка (б)
	//350	Склейка (б)
	//360	Резка(б)
	//370	ПодборкаБлокаКОбложке (об)
	//380	КрышкоВставка (об)

	//449	Ожидает ОТК
	//450	ОТК
	//460	Упаковка
	//465	Отправлен
	//466	Отправлен (сайт)

	//StateCanceledWeb represent photocycle state
	StateCanceledWeb = 505 //Отменен синхронизацией

	//StateCanceled represent photocycle state
	StateCanceled = 507 //Отменен

	//StateCanceledPHCycle represent photocycle state
	StateCanceledPHCycle = 510 //Отменен оператором

	//511	Группа объединена

	//StateCanceledPoduction represent photocycle state
	StateCanceledPoduction = 515 //Отменен производство

	//StateSkiped represent photocycle state
	StateSkiped = 520 //Пропущен
)
