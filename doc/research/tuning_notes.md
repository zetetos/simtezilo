# Notes

## Signal dynamic range compression

### Current variable pulse frequency version


| Device | Stereo |  Mono  |
|--------|--------|--------|
| Master |  -14.5 |  -14.5 |
| Seat   |   34   |   40   | 
| Pedal  |   28   |   40   |


## Current data

+----------+---------+----------+----------+----------+---------+---------------------------------------------------------+
| Version  | Amp Exp | Freq Exp | Channels | Base FFB | GT7 FFB |                          Notes                          |
|----------+---------+----------+----------+----------+---------+---------------------------------------------------------|
| 9421fc5! |   190   |   310    |  Stereo  |   Auto   |    8    | Strong feedback, steering FFB a bit weak in comparison  |
| 9421fc5! |   210   |   310    |  Stereo  |   Auto   |    8    | Strong feedback, fairly well balanced with steering FFB |
| 9421fc5! |   190   |   350    |  Stereo  |   Auto   |    8    | Good general feeback, maybe a bit too compliant         |
| 9421fc5! |   190   |   370    |  Stereo  |   Auto   |    8    | Mild feedback, very compliant                           | 
+----------+---------+----------+----------+----------+---------+---------------------------------------------------------+


## Older data

| Function | Scale  | Channels | Comments                                                 |
|----------|--------|---------------------------------------------------------------------|
| Exp 0.40 | 0.033  |  Mono    | Too harsh, probably needs smaller scale                  |
|*Exp 0.44 | 0.0352 |  Mono    | Pretty good, possibly on the harsh side                  |****
| Exp 0.46 | 0.036  |  Stereo  | Less dynamic range but more feedback, small bumps too    |
|          |        |          | strong compared to steering feedback, missing low freq   |
| Exp 0.46 | 0.0355 |  Stereo  | Less dynamic range but more feedback, small bumps harsh  |
|*Exp 0.46 | 0.0354 |  Stereo  | Less dynamic range but more feedback, slightly harsh     |
|*Exp 0.46 | 0.0353 |  Stereo  | Less dynamic range but more feedback, small bumps about  |
|          |        |          | right with lower frequencies present                     |
|*Exp 0.46 | 0.0353 |  Mono    | Too harsh                                                |
|*Exp 0.46 | 0.0352 |  Stereo  | Less dynamic range but more feedback, slightly weak      |
| Exp 0.46 | 0.035  |  Stereo  | Less dynamic range but more feedback, small bumps a bit  |
|          |        |          | too weak compared to steering feedback                   |
| Exp 0.50 | 0.03   |  Stereo  | Good range, maybe a bit harsh for smaller bumps          |
|*Exp 0.50 | 0.028  |  Stereo  | Good range, smaller bumps fairly subtle                  |
| Exp 0.55 | 0.02   |  Stereo  | reasonably good, small bumps still a bit too soft        |
| Exp 0.56 | 0.02   |  Stereo  | reasonably good, small bumps still a bit too soft        |
| Exp 0.56 | 1/54   |  Stereo  | nuanced small bumps, lacking impact on ripple strips     |

## Older fixed pulse frequency version

| Function | Scale | Comments                                                 |
|----------|-------|----------------------------------------------------------|
| Exp 0.50 | 1/57  | small bumps slightly too loud                            |
| Log10    | 0.08  | small bumps too loud                                     |
| Log2     | 0.025 | small bumps too loud                                     |


## Signal ranges
|  Class  |    Signal    |     Jerk      |      Snap     |
|---------|--------------|---------------|---------------|
| Group B | Road noise   |        50-100 |    5000-15000 |
| Group B | Road bump    |   50-100-2000 |   20000-60000 |
| Group B | Ripple strip | 500-1000-3000 | 100000-200000 |


expS = exp * 0.4
scaleS = (scale^exp) * 0.5
exp
- decrease: sharper ramp up of lower values and gentler roll off of higher values. road noise and ripple strip volumes closer together
- increase: gentler ramper up of lower values and sharper roll off of high values. road noise reduced and ripple strip increased
scale
- decrease: lower all values and increase jerk value where clipping occurs
- increase: raise all values and decrese jerk value where clipping occurs

GT7 FF: 8

Settings:
    Feedback: possibly set range 1-10 with 0.05 exp increments between 0.50 and 1.00
    What?: range 1-10 scale with ? increments between ? and ?

scale   exp     expS    scaleS      gain    notes
0.036   0.50    0.25    0.09487     -15.0   
0.036   0.60    0.30    0.06804     -15.0   very firm, seems to be a good mix with wheel feedback
0.036   0.70    0.35    0.04880     -15.0   quite firm, starting to get good impact feedback as well
0.036   0.80    0.40    0.03500     -15.0   starting to get firm, still limited feedback over ripple stips
0.036   0.90    0.45    0.02510     -15.0   still fairly compliant with less overall feedback
0.036   1.00    0.50    0.01800     -15.0   very compliant, less feedback, feels like more supple tyres

0.036   0.73    0.365   0.04416     -15.0   good range, subtle road noise and good bump/ripple. Not fatiquing RECHECK
0.036   0.76    0.38    0.03997     -15.0   more road noise, closer overlap with bump/ripple, good match to wheel feedback RECHECK


0.036   0.728   0.364   0.48560     -14.5   good range but quite harsh even at -15db
0.036   0.95    0.380   0.04251     -14.5   very good range, subtle road noise and harsh bump/ripple but gearchange also harsh, maybe reduce gain

