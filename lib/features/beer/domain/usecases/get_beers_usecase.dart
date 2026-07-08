import 'package:fpdart/fpdart.dart';

import '../../../../core/error/failures.dart';
import '../entities/beer.dart';
import '../repositories/i_beer_repository.dart';
import '../../../../core/usecase/usecase.dart';

class GetBeersUseCase implements UseCase<NoParams, List<Beer>> {
  GetBeersUseCase({required this.beerRepository});

  final IBeerRepository beerRepository;

  @override
  Future<Either<Failure, List<Beer>>> call(NoParams params) async {
    return beerRepository.getBeers();
  }
}
