// Package rating — рейтинг по темам на основе Glicko-2 (Марк Глик, glicko2.pdf).
// Рейтинг хранится парой «оценка + неуверенность»: чем реже тема тренировалась, тем шире разброс.
// Все функции чистые: ни времени, ни случайности, ни ввода-вывода.
package rating

import "math"

// Tau — ограничение скорости изменения волатильности (в статье рекомендуется 0,3–1,2).
const Tau = 0.5

const (
	scale     = 173.7178 // перевод между шкалой 1500 и внутренней шкалой Glicko-2
	baseMu    = 1500.0
	maxPhi    = 350.0
	tolerance = 1e-6
)

// Rating — состояние рейтинга темы.
type Rating struct {
	Mu    float64 // рейтинг на шкале 1500
	Phi   float64 // отклонение (неуверенность)
	Sigma float64 // волатильность
}

// New — рейтинг новой темы: 1500, отклонение 350, волатильность 0,06.
func New() Rating { return Rating{Mu: baseMu, Phi: maxPhi, Sigma: 0.06} }

type match struct {
	opp   Rating
	score float64
}

// Update — один матч за период; score 1 — верно, 0 — неверно.
func Update(r Rating, opponent Rating, score float64) Rating {
	return updatePeriod(r, []match{{opponent, score}})
}

func g(phi float64) float64 { return 1 / math.Sqrt(1+3*phi*phi/(math.Pi*math.Pi)) }

func expect(mu, muJ, phiJ float64) float64 { return 1 / (1 + math.Exp(-g(phiJ)*(mu-muJ))) }

// updatePeriod — шаги 2–8 из статьи Глика для набора матчей одного периода.
func updatePeriod(r Rating, ms []match) Rating {
	// Шаг 2: перевод на внутреннюю шкалу.
	mu := (r.Mu - baseMu) / scale
	phi := r.Phi / scale
	sigma := r.Sigma

	if len(ms) == 0 {
		// Без матчей меняется только отклонение (шаг 6).
		return Rating{Mu: r.Mu, Phi: math.Min(math.Sqrt(phi*phi+sigma*sigma)*scale, maxPhi), Sigma: sigma}
	}

	// Шаги 3–4: оценочная дисперсия v и улучшение delta.
	var vInv, sum float64
	for _, m := range ms {
		muJ := (m.opp.Mu - baseMu) / scale
		phiJ := m.opp.Phi / scale
		gj := g(phiJ)
		e := expect(mu, muJ, phiJ)
		vInv += gj * gj * e * (1 - e)
		sum += gj * (m.score - e)
	}
	v := 1 / vInv
	delta := v * sum

	// Шаг 5: новая волатильность, итерация Иллинойс.
	a := math.Log(sigma * sigma)
	f := func(x float64) float64 {
		ex := math.Exp(x)
		d := phi*phi + v + ex
		return ex*(delta*delta-phi*phi-v-ex)/(2*d*d) - (x-a)/(Tau*Tau)
	}
	A := a
	var B float64
	if delta*delta > phi*phi+v {
		B = math.Log(delta*delta - phi*phi - v)
	} else {
		k := 1.0
		for f(a-k*Tau) < 0 {
			k++
		}
		B = a - k*Tau
	}
	fA, fB := f(A), f(B)
	for math.Abs(B-A) > tolerance {
		C := A + (A-B)*fA/(fB-fA)
		fC := f(C)
		if fC*fB <= 0 {
			A, fA = B, fB
		} else {
			fA /= 2
		}
		B, fB = C, fC
	}
	newSigma := math.Exp(A / 2)

	// Шаги 6–7: новое отклонение и рейтинг.
	phiStar := math.Sqrt(phi*phi + newSigma*newSigma)
	newPhi := 1 / math.Sqrt(1/(phiStar*phiStar)+1/v)
	newMu := mu + newPhi*newPhi*sum

	// Шаг 8: обратно на шкалу 1500; отклонение не выше начального.
	return Rating{
		Mu:    newMu*scale + baseMu,
		Phi:   math.Min(newPhi*scale, maxPhi),
		Sigma: newSigma,
	}
}
